package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"gmc/api/internal/auth"
	"gmc/api/internal/gen/db"
	"gmc/api/internal/gen/openapi"
	"gmc/api/internal/inventory"
	"gmc/api/internal/platform"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type importDryRunToken struct {
	csvHash   string
	expiresAt time.Time
}

type Server struct {
	pool              *pgxpool.Pool
	q                 *db.Queries
	cfg               *platform.Config
	cache             *platform.InMemoryCache
	reval             *platform.Revalidator
	authSvc           *auth.Service
	invSvc            *inventory.Service
	storage           platform.Storage
	metrics           *platform.MetricsCollector
	loginLimiter      *platform.KeyedRateLimiter
	registerLimiter   *platform.KeyedRateLimiter
	forgotLimiter     *platform.KeyedRateLimiter
	contactLimiter    *platform.KeyedRateLimiter
	newsletterLimiter *platform.KeyedRateLimiter
	checkoutLimiter   *platform.KeyedRateLimiter
	mu                sync.Mutex
	importTokens      map[string]importDryRunToken
}

func NewServer(pool *pgxpool.Pool, cfg *platform.Config, storage platform.Storage, metrics *platform.MetricsCollector) *Server {
	return &Server{
		pool:              pool,
		q:                 db.New(pool),
		cfg:               cfg,
		cache:             platform.NewInMemoryCache(),
		reval:             platform.NewRevalidator(cfg.WebInternalURL, cfg.RevalidateSecret),
		authSvc:           auth.NewService(pool, cfg),
		invSvc:            inventory.NewService(),
		storage:           storage,
		metrics:           metrics,
		loginLimiter:      platform.NewKeyedRateLimiter(5, 5),
		registerLimiter:   platform.NewKeyedRateLimiter(3, 3),
		forgotLimiter:     platform.NewKeyedRateLimiter(3, 3),
		contactLimiter:    platform.NewKeyedRateLimiter(5, 5),
		newsletterLimiter: platform.NewKeyedRateLimiter(5, 5),
		checkoutLimiter:   platform.NewKeyedRateLimiter(10, 10),
		importTokens:      make(map[string]importDryRunToken),
	}
}

func (s *Server) InvalidateCatalogCache() {
	s.cache.DeletePrefix("catalog:")
	s.reval.Trigger("catalog")
}

func (s *Server) InvalidateSiteCache() {
	s.cache.DeletePrefix("site:")
	s.reval.Trigger("site")
}

// ServeUploads serves processed images at /uploads/{id}/{size} with immutable caching.
func (s *Server) ServeUploads(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	size := chi.URLParam(r, "size")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	f, err := s.storage.OpenVariant(id, size)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	_, _ = io.Copy(w, f)
}

// ServeMetrics exposes Prometheus-format metrics at /metrics.
func (s *Server) ServeMetrics(w http.ResponseWriter, r *http.Request) {
	hits, misses := s.cache.Stats()
	stat := s.pool.Stat()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	fmt.Fprintf(w, "# HELP gmc_http_requests_total Total HTTP requests\n")
	fmt.Fprintf(w, "# TYPE gmc_http_requests_total counter\n")
	fmt.Fprintf(w, "gmc_http_requests_total %d\n", s.metrics.TotalRequests.Load())
	fmt.Fprintf(w, "# HELP gmc_http_errors_total Total 5xx HTTP responses\n")
	fmt.Fprintf(w, "# TYPE gmc_http_errors_total counter\n")
	fmt.Fprintf(w, "gmc_http_errors_total %d\n", s.metrics.TotalErrors.Load())
	fmt.Fprintf(w, "# HELP gmc_cache_hits_total Total cache hits\n")
	fmt.Fprintf(w, "# TYPE gmc_cache_hits_total counter\n")
	fmt.Fprintf(w, "gmc_cache_hits_total %d\n", hits)
	fmt.Fprintf(w, "# HELP gmc_cache_misses_total Total cache misses\n")
	fmt.Fprintf(w, "# TYPE gmc_cache_misses_total counter\n")
	fmt.Fprintf(w, "gmc_cache_misses_total %d\n", misses)
	fmt.Fprintf(w, "# HELP gmc_db_pool_acquired_conns Acquired DB connections\n")
	fmt.Fprintf(w, "# TYPE gmc_db_pool_acquired_conns gauge\n")
	fmt.Fprintf(w, "gmc_db_pool_acquired_conns %d\n", stat.AcquiredConns())
	fmt.Fprintf(w, "# HELP gmc_revalidate_failures_total Total failed ISR revalidation webhooks\n")
	fmt.Fprintf(w, "# TYPE gmc_revalidate_failures_total counter\n")
	fmt.Fprintf(w, "gmc_revalidate_failures_total %d\n", s.reval.Failures())
}

// GetHealthz implements GET /healthz
func (s *Server) GetHealthz(w http.ResponseWriter, r *http.Request) {
	platform.WriteJSON(w, http.StatusOK, openapi.HealthStatus{Status: "ok"})
}

// GetReadyz implements GET /readyz
func (s *Server) GetReadyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 1*time.Second)
	defer cancel()
	if err := s.pool.Ping(ctx); err != nil {
		platform.WriteError(w, r, platform.NewAppError(http.StatusServiceUnavailable, openapi.Internal, "Database belum siap"))
		return
	}
	platform.WriteJSON(w, http.StatusOK, openapi.HealthStatus{Status: "ok"})
}

// GetPublicSite implements GET /v1/site
func (s *Server) GetPublicSite(w http.ResponseWriter, r *http.Request) {
	val, err := s.cache.DoSingleflight("site:public", 60*time.Second, func() (any, error) {
		return s.loadPublicSite(r.Context())
	})
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	platform.WriteCatalogJSON(w, r, val)
}

func (s *Server) loadPublicSite(ctx context.Context) (*openapi.PublicSiteResponse, error) {
	resp := &openapi.PublicSiteResponse{}
	// Defaults from config
	resp.Store.Name = s.cfg.StoreName
	resp.Store.Tagline = "Rakit. Upgrade. Terhubung."
	resp.Store.Address = "ITC Mega Grosir Lt. 2 Blok B/7, Surabaya"
	resp.Store.MapsUrl = "https://www.google.com/maps/search/?api=1&query=ITC+Mega+Grosir+Surabaya"
	resp.Store.Phone = "0822-3255-4674"
	resp.Store.Whatsapp = s.cfg.WhatsAppNumber
	resp.Store.Email = "veneratesman@gmail.com"
	resp.Store.Hours = "Senin – Sabtu, 09.00 – 17.00 WIB"

	resp.Status.IsOpen = true
	resp.Status.Message = "Toko buka normal dan siap melayani pesanan Anda."

	resp.Payment.BankAccounts = []struct {
		Bank   string `json:"bank"`
		Holder string `json:"holder"`
		Number string `json:"number"`
	}{
		{Bank: "BCA (CONTOH)", Number: "0881234567", Holder: "Global Mandiri Computer"},
		{Bank: "Mandiri (CONTOH)", Number: "1420012345678", Holder: "Global Mandiri Computer"},
	}
	resp.Payment.Methods = []string{"bank_transfer", "cod", "pay_at_store"}
	resp.Checkout.AllowGuest = true
	resp.Checkout.MaxQtyPerLine = 99

	// Override with site_settings if present
	settings, err := s.q.ListSiteSettings(ctx)
	if err == nil {
		for _, st := range settings {
			var m map[string]any
			if json.Unmarshal(st.Value, &m) != nil {
				continue
			}
			switch st.Key {
			case "store":
				if v, ok := m["name"].(string); ok && v != "" {
					resp.Store.Name = v
				}
				if v, ok := m["tagline"].(string); ok && v != "" {
					resp.Store.Tagline = v
				}
				if v, ok := m["address"].(string); ok && v != "" {
					resp.Store.Address = v
				}
				if v, ok := m["mapsUrl"].(string); ok && v != "" {
					resp.Store.MapsUrl = v
				}
				if v, ok := m["phone"].(string); ok && v != "" {
					resp.Store.Phone = v
				}
				if v, ok := m["whatsapp"].(string); ok && v != "" {
					resp.Store.Whatsapp = v
				}
				if v, ok := m["email"].(string); ok && v != "" {
					resp.Store.Email = v
				}
				if v, ok := m["hours"].(string); ok && v != "" {
					resp.Store.Hours = v
				}
			case "status":
				if v, ok := m["isOpen"].(bool); ok {
					resp.Status.IsOpen = v
				}
				if v, ok := m["message"].(string); ok {
					resp.Status.Message = v
				}
				if v, ok := m["reopenAt"].(string); ok && v != "" {
					resp.Status.ReopenAt = &v
				}
			case "orders":
				if v, ok := m["allowGuest"].(bool); ok {
					resp.Checkout.AllowGuest = v
				}
				if v, ok := m["maxQtyPerLine"].(float64); ok && v > 0 {
					resp.Checkout.MaxQtyPerLine = int(v)
				}
			case "payment":
				if rawBanks, ok := m["bankAccounts"].([]any); ok && len(rawBanks) > 0 {
					resp.Payment.BankAccounts = nil
					for _, rb := range rawBanks {
						if bm, ok := rb.(map[string]any); ok {
							bank, _ := bm["bank"].(string)
							num, _ := bm["number"].(string)
							holder, _ := bm["holder"].(string)
							resp.Payment.BankAccounts = append(resp.Payment.BankAccounts, struct {
								Bank   string `json:"bank"`
								Holder string `json:"holder"`
								Number string `json:"number"`
							}{Bank: bank, Number: num, Holder: holder})
						}
					}
				}
			}
		}
	}

	anns, _ := s.q.ListActiveAnnouncements(ctx)
	for _, a := range anns {
		resp.Announcements = append(resp.Announcements, struct {
			Id      openapi.UUID `json:"id"`
			LinkUrl *string      `json:"linkUrl"`
			Text    string       `json:"text"`
		}{
			Id:      a.ID,
			Text:    a.Text,
			LinkUrl: a.LinkUrl,
		})
	}
	if resp.Announcements == nil {
		resp.Announcements = []struct {
			Id      openapi.UUID `json:"id"`
			LinkUrl *string      `json:"linkUrl"`
			Text    string       `json:"text"`
		}{}
	}

	slides, _ := s.q.ListActivePromoSlides(ctx)
	for _, sl := range slides {
		resp.Slides = append(resp.Slides, struct {
			Body           *string      `json:"body"`
			CtaLabel       *string      `json:"ctaLabel"`
			CtaUrl         *string      `json:"ctaUrl"`
			Headline       string       `json:"headline"`
			Id             openapi.UUID `json:"id"`
			ImageMobileUrl *string      `json:"imageMobileUrl"`
			ImageUrl       *string      `json:"imageUrl"`
			Label          *string      `json:"label"`
		}{
			Id:             sl.ID,
			Label:          sl.Label,
			Headline:       sl.Headline,
			Body:           sl.Body,
			CtaLabel:       sl.CtaLabel,
			CtaUrl:         sl.CtaUrl,
			ImageUrl:       sl.ImageUrl,
			ImageMobileUrl: sl.ImageMobileUrl,
		})
	}
	if resp.Slides == nil {
		resp.Slides = []struct {
			Body           *string      `json:"body"`
			CtaLabel       *string      `json:"ctaLabel"`
			CtaUrl         *string      `json:"ctaUrl"`
			Headline       string       `json:"headline"`
			Id             openapi.UUID `json:"id"`
			ImageMobileUrl *string      `json:"imageMobileUrl"`
			ImageUrl       *string      `json:"imageUrl"`
			Label          *string      `json:"label"`
		}{}
	}

	faqs, _ := s.q.ListActiveFaqs(ctx)
	for _, fq := range faqs {
		resp.Faqs = append(resp.Faqs, struct {
			Anchor   string       `json:"anchor"`
			Answer   string       `json:"answer"`
			Id       openapi.UUID `json:"id"`
			Question string       `json:"question"`
		}{
			Id:       fq.ID,
			Anchor:   fq.Anchor,
			Question: fq.Question,
			Answer:   fq.Answer,
		})
	}
	if resp.Faqs == nil {
		resp.Faqs = []struct {
			Anchor   string       `json:"anchor"`
			Answer   string       `json:"answer"`
			Id       openapi.UUID `json:"id"`
			Question string       `json:"question"`
		}{}
	}

	return resp, nil
}

// StartAutoCancelWorker runs periodically under pg_try_advisory_lock to cancel expired awaiting-confirmation orders and restore stock.
func (s *Server) StartAutoCancelWorker(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Minute)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.runAutoCancelPass(ctx)
			}
		}
	}()
}

func (s *Server) runAutoCancelPass(ctx context.Context) {
	hours := 48
	if st, err := s.q.GetSiteSetting(ctx, "orders"); err == nil {
		var m map[string]any
		if json.Unmarshal(st.Value, &m) == nil {
			if h, ok := m["orderAutoCancelHours"].(float64); ok {
				hours = int(h)
			}
		}
	}
	if hours <= 0 {
		return
	}

	var locked bool
	if err := s.pool.QueryRow(ctx, "SELECT pg_try_advisory_lock(8842101)").Scan(&locked); err != nil || !locked {
		return
	}
	defer func() {
		_, _ = s.pool.Exec(context.Background(), "SELECT pg_advisory_unlock(8842101)")
	}()

	cutoff := time.Now().UTC().Add(-time.Duration(hours) * time.Hour)
	rows, err := s.pool.Query(ctx, `
		SELECT order_no FROM orders
		WHERE status = 'menunggu_konfirmasi' AND created_at < $1
		LIMIT 25
	`, cutoff)
	if err != nil {
		return
	}
	var orderNos []string
	for rows.Next() {
		var no string
		if rows.Scan(&no) == nil {
			orderNos = append(orderNos, no)
		}
	}
	rows.Close()

	for _, no := range orderNos {
		reason := fmt.Sprintf("Dibatalkan otomatis karena belum dikonfirmasi lebih dari %d jam", hours)
		if _, err := s.transitionOrderInternal(ctx, no, "dibatalkan", &reason, &reason, nil); err != nil {
			slog.Warn("auto-cancel order failed", "orderNo", no, "err", err)
		}
	}
}

func writeCSVDownload(w http.ResponseWriter, r *http.Request, filename string, delimiterParam *string, headers []string, rows [][]string) {
	sep := ","
	if delimiterParam != nil && strings.EqualFold(*delimiterParam, "semicolon") {
		sep = ";"
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusOK)

	// Write UTF-8 BOM for Excel compatibility
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})

	writeRow := func(cols []string) {
		escaped := make([]string, len(cols))
		for i, c := range cols {
			c = platform.EscapeCSVCell(c)
			if strings.ContainsAny(c, `",;`+"\n\r") {
				c = `"` + strings.ReplaceAll(c, `"`, `""`) + `"`
			}
			escaped[i] = c
		}
		_, _ = io.WriteString(w, strings.Join(escaped, sep)+"\r\n")
	}

	writeRow(headers)
	for i, row := range rows {
		if i >= 50000 {
			break
		}
		writeRow(row)
	}
}
