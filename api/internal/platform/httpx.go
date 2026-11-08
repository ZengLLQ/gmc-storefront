package platform

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gmc/api/internal/gen/openapi"

	"golang.org/x/time/rate"
)

type ctxKey string

const (
	ctxKeyRequestID ctxKey = "req_id"
	ctxKeyClientIP  ctxKey = "client_ip"
	ctxKeyActor     ctxKey = "actor"
)

type Actor struct {
	UserID    string
	Email     string
	Role      string
	SessionID string
	IsAdmin   bool
}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyRequestID, id)
}

func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyRequestID).(string); ok && v != "" {
		return v
	}
	return NewUUID().String()
}

func WithClientIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, ctxKeyClientIP, ip)
}

func ClientIPFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyClientIP).(string); ok && v != "" {
		return v
	}
	return "127.0.0.1"
}

func WithActor(ctx context.Context, a *Actor) context.Context {
	return context.WithValue(ctx, ctxKeyActor, a)
}

func ActorFromContext(ctx context.Context) *Actor {
	if v, ok := ctx.Value(ctxKeyActor).(*Actor); ok {
		return v
	}
	return nil
}

type AppError struct {
	Status  int
	Code    openapi.ErrorCode
	Message string
	Fields  map[string]string
}

func (e *AppError) Error() string {
	return fmt.Sprintf("%s (%d): %s", e.Code, e.Status, e.Message)
}

func NewAppError(status int, code openapi.ErrorCode, message string) *AppError {
	return &AppError{Status: status, Code: code, Message: message}
}

func NewValidationError(message string, fields map[string]string) *AppError {
	if message == "" {
		message = "Data tidak valid"
	}
	return &AppError{
		Status:  http.StatusUnprocessableEntity,
		Code:    openapi.ValidationFailed,
		Message: message,
		Fields:  fields,
	}
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func WriteCatalogJSON(w http.ResponseWriter, r *http.Request, v any) {
	raw, err := json.Marshal(v)
	if err != nil {
		WriteError(w, r, NewAppError(http.StatusInternalServerError, openapi.Internal, "Terjadi kesalahan internal"))
		return
	}
	sum := sha1.Sum(raw)
	etag := `"` + hex.EncodeToString(sum[:10]) + `"`
	w.Header().Set("Cache-Control", "public, s-maxage=60, stale-while-revalidate=300")
	w.Header().Set("ETag", etag)
	if inm := r.Header.Get("If-None-Match"); inm != "" && inm == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}

func WritePrivateJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Cache-Control", "private, no-store")
	WriteJSON(w, status, v)
}

func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	reqID := RequestIDFromContext(r.Context())
	w.Header().Set("Cache-Control", "private, no-store")

	var appErr *AppError
	if errors.As(err, &appErr) {
		env := openapi.ErrorEnvelope{}
		env.Error.Code = appErr.Code
		env.Error.Message = appErr.Message
		env.Error.RequestId = reqID
		if appErr.Code == openapi.ValidationFailed && len(appErr.Fields) > 0 {
			fieldsCopy := make(map[string]string, len(appErr.Fields))
			for k, v := range appErr.Fields {
				fieldsCopy[k] = v
			}
			env.Error.Fields = &fieldsCopy
		}
		WriteJSON(w, appErr.Status, env)
		return
	}

	slog.Error("unhandled server error", "requestId", reqID, "err", err)
	env := openapi.ErrorEnvelope{}
	env.Error.Code = openapi.Internal
	env.Error.Message = "Terjadi kesalahan pada server"
	env.Error.RequestId = reqID
	WriteJSON(w, http.StatusInternalServerError, env)
}

// DecodeStrictJSON decodes JSON with a 1 MB limit and rejects unknown fields.
func DecodeStrictJSON(r *http.Request, dst any) error {
	body := http.MaxBytesReader(nil, r.Body, 1<<20)
	dec := json.NewDecoder(body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return NewAppError(http.StatusBadRequest, openapi.BadRequest, "Format JSON tidak valid atau terdapat atribut yang tidak dikenal")
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return NewAppError(http.StatusBadRequest, openapi.BadRequest, "Hanya satu objek JSON yang diperbolehkan")
	}
	return nil
}

// ExtractRealIP extracts the client IP, honoring X-Forwarded-For only if RemoteAddr is in trustedNets.
func ExtractRealIP(r *http.Request, trustedNets []*net.IPNet) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	remoteIP := net.ParseIP(host)
	isTrusted := false
	if remoteIP != nil {
		for _, n := range trustedNets {
			if n.Contains(remoteIP) {
				isTrusted = true
				break
			}
		}
	}
	if isTrusted {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			first := strings.TrimSpace(parts[0])
			if parsed := net.ParseIP(first); parsed != nil {
				return parsed.String()
			}
		}
		if xrip := strings.TrimSpace(r.Header.Get("X-Real-IP")); xrip != "" {
			if parsed := net.ParseIP(xrip); parsed != nil {
				return parsed.String()
			}
		}
	}
	if remoteIP != nil {
		return remoteIP.String()
	}
	return "127.0.0.1"
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// MetricsCollector tracks request counts and latencies for /metrics.
type MetricsCollector struct {
	TotalRequests atomic.Uint64
	TotalErrors   atomic.Uint64
}

func StandardMiddlewares(cfg *Config, metrics *MetricsCollector) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			reqID := strings.TrimSpace(r.Header.Get("X-Request-Id"))
			if len(reqID) < 8 || len(reqID) > 64 {
				reqID = NewUUID().String()
			}
			clientIP := ExtractRealIP(r, cfg.TrustedProxies)

			ctx := WithRequestID(r.Context(), reqID)
			ctx = WithClientIP(ctx, clientIP)

			// 10s request deadline
			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			w.Header().Set("X-Request-Id", reqID)
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Cross-Origin-Resource-Policy", "same-site")

			// Optional CORS escape hatch
			if len(cfg.CORSAllowed) > 0 {
				origin := r.Header.Get("Origin")
				for _, allowed := range cfg.CORSAllowed {
					if allowed == "*" || strings.EqualFold(allowed, origin) {
						w.Header().Set("Access-Control-Allow-Origin", origin)
						w.Header().Set("Access-Control-Allow-Credentials", "true")
						w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Requested-With, Idempotency-Key")
						w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
						break
					}
				}
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}

			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			defer func() {
				if rec := recover(); rec != nil {
					slog.Error("panic recovered",
						"requestId", reqID,
						"panic", rec,
						"stack", string(debug.Stack()),
					)
					WriteError(sw, r.WithContext(ctx), NewAppError(http.StatusInternalServerError, openapi.Internal, "Terjadi kesalahan internal"))
				}
				durMs := time.Since(start).Milliseconds()
				if metrics != nil {
					metrics.TotalRequests.Add(1)
					if sw.status >= 500 {
						metrics.TotalErrors.Add(1)
					}
				}
				attrs := []any{
					"requestId", reqID,
					"method", r.Method,
					"path", r.URL.Path,
					"status", sw.status,
					"durationMs", durMs,
					"ip", clientIP,
				}
				if actor := ActorFromContext(ctx); actor != nil {
					attrs = append(attrs, "userId", actor.UserID)
				}
				slog.Info("http_request", attrs...)
			}()

			// CSRF protection on all mutating requests
			if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch || r.Method == http.MethodDelete {
				if !strings.HasPrefix(r.URL.Path, "/internal/") {
					xrw := r.Header.Get("X-Requested-With")
					if xrw != "gmc-web" {
						WriteError(sw, r.WithContext(ctx), NewAppError(http.StatusForbidden, openapi.Forbidden, "Header keamanan X-Requested-With tidak valid"))
						return
					}
					origin := r.Header.Get("Origin")
					if origin == "" {
						if ref := r.Header.Get("Referer"); ref != "" {
							if u, err := url.Parse(ref); err == nil {
								origin = u.Scheme + "://" + u.Host
							}
						}
					}
					originOK := false
					for _, allowed := range cfg.AllowedOrigins {
						if strings.EqualFold(strings.TrimRight(allowed, "/"), strings.TrimRight(origin, "/")) {
							originOK = true
							break
						}
					}
					if !originOK {
						WriteError(sw, r.WithContext(ctx), NewAppError(http.StatusForbidden, openapi.Forbidden, "Origin permintaan tidak diizinkan"))
						return
					}
				}
			}

			next.ServeHTTP(sw, r.WithContext(ctx))
		})
	}
}

type KeyedRateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	r        rate.Limit
	burst    int
}

func NewKeyedRateLimiter(perMinute int, burst int) *KeyedRateLimiter {
	return &KeyedRateLimiter{
		limiters: make(map[string]*rate.Limiter),
		r:        rate.Every(time.Minute / time.Duration(perMinute)),
		burst:    burst,
	}
}

func (l *KeyedRateLimiter) Allow(key string) bool {
	l.mu.Lock()
	lim, ok := l.limiters[key]
	if !ok {
		lim = rate.NewLimiter(l.r, l.burst)
		l.limiters[key] = lim
	}
	l.mu.Unlock()
	return lim.Allow()
}

func RateLimitMiddleware(limiter *KeyedRateLimiter, retryAfterSec int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := ClientIPFromContext(r.Context())
			if !limiter.Allow(ip) {
				w.Header().Set("Retry-After", strconv.Itoa(retryAfterSec))
				WriteError(w, r, NewAppError(http.StatusTooManyRequests, openapi.RateLimited, "Terlalu banyak permintaan, silakan coba beberapa saat lagi"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
