package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gmc/api/internal/admin/rbac"
	"gmc/api/internal/audit"
	"gmc/api/internal/auth"
	"gmc/api/internal/gen/db"
	"gmc/api/internal/gen/openapi"
	"gmc/api/internal/inventory"
	"gmc/api/internal/platform"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/pquerna/otp/totp"
)

// AdminLogin implements POST /v1/admin/auth/login
func (s *Server) AdminLogin(w http.ResponseWriter, r *http.Request) {
	ip := platform.ClientIPFromContext(r.Context())
	var req openapi.AdminLoginRequest
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	email := strings.ToLower(strings.TrimSpace(string(req.Email)))
	lockKey := ip + ":" + email

	if err := s.authSvc.CheckAdminLockout(lockKey); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	if !s.loginLimiter.Allow(lockKey) {
		platform.WriteError(w, r, platform.NewAppError(http.StatusTooManyRequests, openapi.RateLimited, "Terlalu banyak percobaan login admin, coba lagi dalam 1 menit"))
		return
	}

	user, err := s.q.GetUserByEmail(r.Context(), email)
	if err != nil || !user.IsActive || (user.Role != "admin" && user.Role != "staff") || !platform.VerifyPassword(req.Password, user.PasswordHash) {
		s.authSvc.RecordAdminLoginFailure(r.Context(), lockKey, email, ip)
		platform.WriteError(w, r, platform.NewAppError(http.StatusUnauthorized, openapi.InvalidCredentials, "Email atau password salah"))
		return
	}

	// Check if 2FA is enabled for this user
	totpRec, totpErr := s.q.GetUserTotp(r.Context(), user.ID)
	has2FA := totpErr == nil && totpRec.ConfirmedAt != nil

	if has2FA {
		if req.TotpCode == nil || strings.TrimSpace(*req.TotpCode) == "" {
			req2fa := true
			platform.WritePrivateJSON(w, http.StatusOK, openapi.AdminAuthResponse{
				Requires2fa: &req2fa,
			})
			return
		}
		ok, err := s.authSvc.VerifyTOTPOrBackup(r.Context(), user.ID, *req.TotpCode)
		if err != nil || !ok {
			s.authSvc.RecordAdminLoginFailure(r.Context(), lockKey, email, ip)
			platform.WriteError(w, r, platform.NewAppError(http.StatusUnauthorized, openapi.InvalidCredentials, "Kode autentikasi 2FA tidak valid"))
			return
		}
	}

	s.authSvc.ClearAdminLoginFailures(lockKey)
	accessJWT, refreshRaw, err := s.authSvc.IssueSession(r.Context(), user, true, r.UserAgent(), ip)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	s.authSvc.SetAuthCookies(w, accessJWT, refreshRaw, true)

	_ = audit.Record(r.Context(), s.q, audit.Entry{
		ActorID:    uuid.NullUUID{UUID: user.ID, Valid: true},
		ActorEmail: user.Email,
		ActorRole:  user.Role,
		Action:     "admin.auth.login",
		EntityType: "user",
		EntityID:   user.ID.String(),
		Summary:    fmt.Sprintf("Login admin berhasil (%s)", user.Email),
		IP:         ip,
	})

	u := toOpenAPIUser(user)
	perms := rbac.PermissionStringsForRole(user.Role)
	platform.WritePrivateJSON(w, http.StatusOK, openapi.AdminAuthResponse{
		User:        &u,
		Permissions: &perms,
		TotpEnabled: &has2FA,
	})
}

// AdminLogout implements POST /v1/admin/auth/logout
func (s *Server) AdminLogout(w http.ResponseWriter, r *http.Request) {
	s.LogoutCustomer(w, r)
}

// AdminRefresh implements POST /v1/admin/auth/refresh
func (s *Server) AdminRefresh(w http.ResponseWriter, r *http.Request) {
	s.RefreshCustomerSession(w, r)
}

// GetAdminMe implements GET /v1/admin/me
func (s *Server) GetAdminMe(w http.ResponseWriter, r *http.Request) {
	_, user, ok := s.authSvc.RequirePermission(w, r, rbac.PermAuthenticated)
	if !ok {
		return
	}
	totpRec, totpErr := s.q.GetUserTotp(r.Context(), user.ID)
	has2FA := totpErr == nil && totpRec.ConfirmedAt != nil
	u := toOpenAPIUser(*user)
	perms := rbac.PermissionStringsForRole(user.Role)
	platform.WritePrivateJSON(w, http.StatusOK, openapi.AdminAuthResponse{
		User:        &u,
		Permissions: &perms,
		TotpEnabled: &has2FA,
	})
}

// UpdateAdminMe implements PATCH /v1/admin/me
func (s *Server) UpdateAdminMe(w http.ResponseWriter, r *http.Request) {
	_, user, ok := s.authSvc.RequirePermission(w, r, rbac.PermAuthenticated)
	if !ok {
		return
	}
	var req openapi.UpdateProfileRequest
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	phone, err := platform.NormalizePhone(req.Phone)
	if err != nil {
		platform.WriteError(w, r, platform.NewValidationError(err.Error(), map[string]string{"phone": err.Error()}))
		return
	}
	updated, err := s.q.UpdateUserProfile(r.Context(), db.UpdateUserProfileParams{
		ID:    user.ID,
		Name:  strings.TrimSpace(req.Name),
		Email: strings.ToLower(strings.TrimSpace(string(req.Email))),
		Phone: phone,
	})
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	_ = audit.Record(r.Context(), s.q, audit.Entry{
		ActorID:    uuid.NullUUID{UUID: user.ID, Valid: true},
		ActorEmail: user.Email,
		ActorRole:  user.Role,
		Action:     "admin.me.update",
		EntityType: "user",
		EntityID:   user.ID.String(),
		Summary:    "Memperbarui profil akun admin sendiri",
	})
	totpRec, totpErr := s.q.GetUserTotp(r.Context(), updated.ID)
	has2FA := totpErr == nil && totpRec.ConfirmedAt != nil
	u := toOpenAPIUser(updated)
	perms := rbac.PermissionStringsForRole(updated.Role)
	platform.WritePrivateJSON(w, http.StatusOK, openapi.AdminAuthResponse{
		User:        &u,
		Permissions: &perms,
		TotpEnabled: &has2FA,
	})
}

// ChangeAdminPassword implements PUT /v1/admin/me/password
func (s *Server) ChangeAdminPassword(w http.ResponseWriter, r *http.Request) {
	actor, user, ok := s.authSvc.RequirePermission(w, r, rbac.PermAuthenticated)
	if !ok {
		return
	}
	var req openapi.ChangePasswordRequest
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	if !platform.VerifyPassword(req.CurrentPassword, user.PasswordHash) {
		platform.WriteError(w, r, platform.NewAppError(http.StatusUnauthorized, openapi.InvalidCredentials, "Password saat ini salah"))
		return
	}
	if err := platform.ValidatePasswordStrength(req.NewPassword); err != nil {
		platform.WriteError(w, r, platform.NewValidationError(err.Error(), map[string]string{"newPassword": err.Error()}))
		return
	}
	pwHash, err := platform.HashPassword(req.NewPassword)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	if err := s.q.UpdateUserPassword(r.Context(), db.UpdateUserPasswordParams{
		ID:           user.ID,
		PasswordHash: pwHash,
	}); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	if sid, err := uuid.Parse(actor.SessionID); err == nil {
		_ = s.q.RevokeUserSessionsExcept(r.Context(), db.RevokeUserSessionsExceptParams{
			UserID: user.ID,
			ID:     sid,
		})
	}
	_ = audit.Record(r.Context(), s.q, audit.Entry{
		ActorID:    uuid.NullUUID{UUID: user.ID, Valid: true},
		ActorEmail: user.Email,
		ActorRole:  user.Role,
		Action:     "admin.me.password_change",
		EntityType: "user",
		EntityID:   user.ID.String(),
		Summary:    "Mengubah password akun admin dan mencabut sesi lain",
	})
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusNoContent)
}

// SetupAdmin2fa implements POST /v1/admin/me/2fa/setup
func (s *Server) SetupAdmin2fa(w http.ResponseWriter, r *http.Request) {
	_, user, ok := s.authSvc.RequirePermission(w, r, rbac.PermAuthenticated)
	if !ok {
		return
	}
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      s.cfg.StoreName,
		AccountName: user.Email,
	})
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	encSecret, err := platform.EncryptAESGCM(s.cfg.TOTPEncKey, []byte(key.Secret()))
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	if err := s.q.UpsertUserTotp(r.Context(), db.UpsertUserTotpParams{
		UserID:          user.ID,
		SecretEnc:       encSecret,
		ConfirmedAt:     nil,
		BackupCodesHash: [][]byte{},
	}); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	platform.WritePrivateJSON(w, http.StatusOK, map[string]string{
		"secret":     key.Secret(),
		"otpauthUrl": key.URL(),
	})
}

// ConfirmAdmin2fa implements POST /v1/admin/me/2fa/confirm
func (s *Server) ConfirmAdmin2fa(w http.ResponseWriter, r *http.Request) {
	_, user, ok := s.authSvc.RequirePermission(w, r, rbac.PermAuthenticated)
	if !ok {
		return
	}
	var req struct {
		Code string `json:"code"`
	}
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	rec, err := s.q.GetUserTotp(r.Context(), user.ID)
	if err != nil {
		platform.WriteError(w, r, platform.NewAppError(http.StatusBadRequest, openapi.BadRequest, "Setup 2FA belum dimulai"))
		return
	}
	secretBytes, err := platform.DecryptAESGCM(s.cfg.TOTPEncKey, rec.SecretEnc)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	if !totp.Validate(strings.TrimSpace(req.Code), string(secretBytes)) {
		platform.WriteError(w, r, platform.NewValidationError("Kode 2FA tidak cocok", map[string]string{"code": "Kode 6 digit tidak valid"}))
		return
	}
	plainCodes, hashedCodes, err := auth.GenerateBackupCodes()
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	now := time.Now().UTC()
	if err := s.q.UpsertUserTotp(r.Context(), db.UpsertUserTotpParams{
		UserID:          user.ID,
		SecretEnc:       rec.SecretEnc,
		ConfirmedAt:     &now,
		BackupCodesHash: hashedCodes,
	}); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	_ = audit.Record(r.Context(), s.q, audit.Entry{
		ActorID:    uuid.NullUUID{UUID: user.ID, Valid: true},
		ActorEmail: user.Email,
		ActorRole:  user.Role,
		Action:     "admin.me.2fa_enable",
		EntityType: "user",
		EntityID:   user.ID.String(),
		Summary:    "Mengaktifkan autentikasi dua faktor (2FA TOTP)",
	})
	platform.WritePrivateJSON(w, http.StatusOK, map[string]any{
		"backupCodes": plainCodes,
	})
}

// DisableAdmin2fa implements POST /v1/admin/me/2fa/disable
func (s *Server) DisableAdmin2fa(w http.ResponseWriter, r *http.Request) {
	_, user, ok := s.authSvc.RequirePermission(w, r, rbac.PermAuthenticated)
	if !ok {
		return
	}
	var req struct {
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	if !platform.VerifyPassword(req.Password, user.PasswordHash) {
		platform.WriteError(w, r, platform.NewAppError(http.StatusUnauthorized, openapi.InvalidCredentials, "Password salah"))
		return
	}
	validCode, _ := s.authSvc.VerifyTOTPOrBackup(r.Context(), user.ID, req.Code)
	if !validCode {
		platform.WriteError(w, r, platform.NewAppError(http.StatusUnauthorized, openapi.InvalidCredentials, "Kode 2FA tidak valid"))
		return
	}
	_ = s.q.DeleteUserTotp(r.Context(), user.ID)
	_ = audit.Record(r.Context(), s.q, audit.Entry{
		ActorID:    uuid.NullUUID{UUID: user.ID, Valid: true},
		ActorEmail: user.Email,
		ActorRole:  user.Role,
		Action:     "admin.me.2fa_disable",
		EntityType: "user",
		EntityID:   user.ID.String(),
		Summary:    "Menonaktifkan autentikasi dua faktor (2FA TOTP)",
	})
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusNoContent)
}

// ListAdminSessions implements GET /v1/admin/me/sessions
func (s *Server) ListAdminSessions(w http.ResponseWriter, r *http.Request) {
	actor, user, ok := s.authSvc.RequirePermission(w, r, rbac.PermAuthenticated)
	if !ok {
		return
	}
	rows, err := s.q.ListActiveAdminSessions(r.Context(), user.ID)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	items := make([]openapi.AdminSessionInfo, 0, len(rows))
	for _, row := range rows {
		ua := ""
		if row.UserAgent != nil {
			ua = *row.UserAgent
		}
		ip := ""
		if row.Ip != nil {
			ip = *row.Ip
		}
		items = append(items, openapi.AdminSessionInfo{
			Id:        row.ID,
			UserAgent: ua,
			Ip:        ip,
			CreatedAt: row.CreatedAt,
			ExpiresAt: row.ExpiresAt,
			IsCurrent: row.ID.String() == actor.SessionID,
		})
	}
	platform.WritePrivateJSON(w, http.StatusOK, map[string]any{"items": items})
}

// RevokeAdminSession implements DELETE /v1/admin/me/sessions/{id}
func (s *Server) RevokeAdminSession(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	_, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermAuthenticated)
	if !ok {
		return
	}
	_ = s.q.RevokeSessionByID(r.Context(), id)
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusNoContent)
}

// GetAdminCounters implements GET /v1/admin/counters
func (s *Server) GetAdminCounters(w http.ResponseWriter, r *http.Request) {
	_, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermAuthenticated)
	if !ok {
		return
	}
	var c openapi.AdminCounters
	ctx := r.Context()
	_ = s.pool.QueryRow(ctx, `SELECT count(*)::int FROM orders WHERE status = 'menunggu_konfirmasi'`).Scan(&c.OrdersPendingConfirmation)
	_ = s.pool.QueryRow(ctx, `SELECT count(*)::int FROM orders WHERE status = 'menunggu_pembayaran'`).Scan(&c.OrdersPendingPayment)
	_ = s.pool.QueryRow(ctx, `SELECT count(*)::int FROM orders WHERE status = 'menunggu_konfirmasi' AND created_at < now() - interval '42 hours'`).Scan(&c.OrdersExpiringSoon)
	_ = s.pool.QueryRow(ctx, `
		SELECT count(*)::int FROM variants v
		JOIN products p ON p.id = v.product_id
		WHERE v.is_active = true AND p.deleted_at IS NULL AND v.is_preorder = false AND v.stock > 0 AND v.stock <= v.low_stock_threshold
	`).Scan(&c.LowStockCount)
	_ = s.pool.QueryRow(ctx, `
		SELECT count(*)::int FROM variants v
		JOIN products p ON p.id = v.product_id
		WHERE v.is_active = true AND p.deleted_at IS NULL AND v.is_preorder = false AND v.stock = 0
	`).Scan(&c.OutOfStockCount)
	_ = s.pool.QueryRow(ctx, `SELECT count(*)::int FROM contact_messages WHERE status = 'new'`).Scan(&c.UnreadMessages)

	platform.WritePrivateJSON(w, http.StatusOK, c)
}

// AdminGlobalSearch implements GET /v1/admin/search
func (s *Server) AdminGlobalSearch(w http.ResponseWriter, r *http.Request, params openapi.AdminGlobalSearchParams) {
	_, user, ok := s.authSvc.RequirePermission(w, r, rbac.PermAuthenticated)
	if !ok {
		return
	}
	q := strings.TrimSpace(params.Q)
	resp := openapi.AdminSearchResponse{
		Orders: []struct {
			CustomerName string              `json:"customerName"`
			OrderNo      string              `json:"orderNo"`
			Status       openapi.OrderStatus `json:"status"`
			TotalIdr     int64               `json:"totalIdr"`
		}{},
		Products: []struct {
			Id           openapi_types.UUID `json:"id"`
			Name         string             `json:"name"`
			PriceFromIdr int64              `json:"priceFromIdr"`
			Sku          string             `json:"sku"`
			Slug         string             `json:"slug"`
		}{},
		Customers: []struct {
			Email string             `json:"email"`
			Id    openapi_types.UUID `json:"id"`
			Name  string             `json:"name"`
			Phone string             `json:"phone"`
		}{},
	}
	if q == "" {
		platform.WritePrivateJSON(w, http.StatusOK, resp)
		return
	}
	pattern := "%" + q + "%"
	ctx := r.Context()

	if rbac.HasPermission(user.Role, rbac.PermOrdersRead) {
		rows, err := s.pool.Query(ctx, `
			SELECT order_no, customer_name, status, total_idr
			FROM orders
			WHERE order_no ILIKE $1 OR customer_name ILIKE $1 OR customer_phone ILIKE $1
			ORDER BY created_at DESC LIMIT 6
		`, pattern)
		if err == nil {
			for rows.Next() {
				var o struct {
					CustomerName string              `json:"customerName"`
					OrderNo      string              `json:"orderNo"`
					Status       openapi.OrderStatus `json:"status"`
					TotalIdr     int64               `json:"totalIdr"`
				}
				var st string
				if rows.Scan(&o.OrderNo, &o.CustomerName, &st, &o.TotalIdr) == nil {
					o.Status = openapi.OrderStatus(st)
					resp.Orders = append(resp.Orders, o)
				}
			}
			rows.Close()
		}
	}

	if rbac.HasPermission(user.Role, rbac.PermProductsRead) {
		rows, err := s.pool.Query(ctx, `
			SELECT p.id, p.slug, p.name,
				COALESCE((SELECT v.sku FROM variants v WHERE v.product_id = p.id ORDER BY v.created_at ASC LIMIT 1), '') AS sku,
				COALESCE((SELECT min(v.price_idr) FROM variants v WHERE v.product_id = p.id), 0) AS price_from
			FROM products p
			WHERE p.deleted_at IS NULL AND (p.name ILIKE $1 OR p.slug ILIKE $1 OR EXISTS (SELECT 1 FROM variants v WHERE v.product_id = p.id AND v.sku ILIKE $1))
			ORDER BY p.updated_at DESC LIMIT 6
		`, pattern)
		if err == nil {
			for rows.Next() {
				var pr struct {
					Id           openapi_types.UUID `json:"id"`
					Name         string             `json:"name"`
					PriceFromIdr int64              `json:"priceFromIdr"`
					Sku          string             `json:"sku"`
					Slug         string             `json:"slug"`
				}
				if rows.Scan(&pr.Id, &pr.Slug, &pr.Name, &pr.Sku, &pr.PriceFromIdr) == nil {
					resp.Products = append(resp.Products, pr)
				}
			}
			rows.Close()
		}
	}

	if rbac.HasPermission(user.Role, rbac.PermCustomersRead) {
		rows, err := s.pool.Query(ctx, `
			SELECT id, name, email, phone
			FROM users
			WHERE role = 'customer' AND (name ILIKE $1 OR email ILIKE $1 OR phone ILIKE $1)
			ORDER BY created_at DESC LIMIT 6
		`, pattern)
		if err == nil {
			for rows.Next() {
				var cu struct {
					Email string             `json:"email"`
					Id    openapi_types.UUID `json:"id"`
					Name  string             `json:"name"`
					Phone string             `json:"phone"`
				}
				if rows.Scan(&cu.Id, &cu.Name, &cu.Email, &cu.Phone) == nil {
					resp.Customers = append(resp.Customers, cu)
				}
			}
			rows.Close()
		}
	}

	platform.WritePrivateJSON(w, http.StatusOK, resp)
}

// GetAdminDashboard implements GET /v1/admin/dashboard
func (s *Server) GetAdminDashboard(w http.ResponseWriter, r *http.Request, params openapi.GetAdminDashboardParams) {
	_, user, ok := s.authSvc.RequirePermission(w, r, rbac.PermReportsView)
	if !ok {
		return
	}
	rng := "30d"
	if params.Range != nil {
		rng = string(*params.Range)
	}
	canFinance := rbac.HasPermission(user.Role, rbac.PermFinanceView)
	cacheKey := fmt.Sprintf("dashboard:%s:%v", rng, canFinance)

	val, err := s.cache.DoSingleflight(cacheKey, 30*time.Second, func() (any, error) {
		return s.computeDashboard(r.Context(), rng, canFinance)
	})
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	platform.WritePrivateJSON(w, http.StatusOK, val)
}

func (s *Server) computeDashboard(ctx context.Context, rng string, canFinance bool) (*openapi.AdminDashboardResponse, error) {
	days := 30
	switch rng {
	case "today":
		days = 1
	case "7d":
		days = 7
	case "90d":
		days = 90
	}
	now := time.Now().UTC()
	from := now.AddDate(0, 0, -days)
	prevFrom := from.AddDate(0, 0, -days)

	var resp openapi.AdminDashboardResponse

	// Confirmed revenue & order count (current vs previous period)
	var prevRev int64
	var prevOrders int
	_ = s.pool.QueryRow(ctx, `
		SELECT
			COALESCE(sum(total_idr) FILTER (WHERE status IN ('diproses','dikirim','siap_diambil','selesai')), 0)::bigint,
			COALESCE(sum(total_idr) FILTER (WHERE status IN ('menunggu_konfirmasi','menunggu_pembayaran')), 0)::bigint,
			count(*)::int
		FROM orders
		WHERE created_at >= $1
	`, from).Scan(&resp.Kpis.ConfirmedRevenueIdr, &resp.Kpis.PipelineRevenueIdr, &resp.Kpis.OrderCount)

	_ = s.pool.QueryRow(ctx, `
		SELECT
			COALESCE(sum(total_idr) FILTER (WHERE status IN ('diproses','dikirim','siap_diambil','selesai')), 0)::bigint,
			count(*)::int
		FROM orders
		WHERE created_at >= $1 AND created_at < $2
	`, prevFrom, from).Scan(&prevRev, &prevOrders)

	if prevRev > 0 {
		resp.Kpis.ConfirmedRevenueDeltaPct = float32(float64(resp.Kpis.ConfirmedRevenueIdr-prevRev) * 100.0 / float64(prevRev))
	}
	if prevOrders > 0 {
		resp.Kpis.OrderCountDeltaPct = float32(float64(resp.Kpis.OrderCount-prevOrders) * 100.0 / float64(prevOrders))
	}
	if resp.Kpis.OrderCount > 0 {
		resp.Kpis.AovIdr = resp.Kpis.ConfirmedRevenueIdr / int64(resp.Kpis.OrderCount)
	}
	_ = s.pool.QueryRow(ctx, `SELECT count(*)::int FROM users WHERE role = 'customer' AND created_at >= $1`, from).Scan(&resp.Kpis.NewCustomers)

	if canFinance {
		var profit int64
		_ = s.pool.QueryRow(ctx, `
			SELECT COALESCE(sum((oi.unit_price_idr - COALESCE(oi.cost_idr, 0)) * oi.quantity), 0)::bigint
			FROM order_items oi
			JOIN orders o ON o.id = oi.order_id
			WHERE o.created_at >= $1 AND o.status IN ('diproses','dikirim','siap_diambil','selesai') AND oi.cost_idr IS NOT NULL
		`, from).Scan(&profit)
		resp.Kpis.EstimatedGrossProfitIdr = &profit
	}

	// Daily series
	dRows, err := s.pool.Query(ctx, `
		SELECT to_char(created_at AT TIME ZONE 'Asia/Jakarta', 'YYYY-MM-DD') AS d,
			COALESCE(sum(total_idr) FILTER (WHERE status <> 'dibatalkan'), 0)::bigint,
			count(*)::int
		FROM orders
		WHERE created_at >= $1
		GROUP BY d
		ORDER BY d ASC
	`, from)
	if err == nil {
		for dRows.Next() {
			var pt struct {
				Date       string `json:"date"`
				Orders     int    `json:"orders"`
				RevenueIdr int64  `json:"revenueIdr"`
			}
			if dRows.Scan(&pt.Date, &pt.RevenueIdr, &pt.Orders) == nil {
				resp.DailySeries = append(resp.DailySeries, pt)
			}
		}
		dRows.Close()
	}
	if resp.DailySeries == nil {
		resp.DailySeries = []struct {
			Date       string `json:"date"`
			Orders     int    `json:"orders"`
			RevenueIdr int64  `json:"revenueIdr"`
		}{}
	}

	// Status breakdown
	sRows, err := s.pool.Query(ctx, `SELECT status, count(*)::int FROM orders GROUP BY status ORDER BY count(*) DESC`)
	if err == nil {
		for sRows.Next() {
			var sb struct {
				Count  int    `json:"count"`
				Status string `json:"status"`
			}
			if sRows.Scan(&sb.Status, &sb.Count) == nil {
				resp.StatusBreakdown = append(resp.StatusBreakdown, sb)
			}
		}
		sRows.Close()
	}
	if resp.StatusBreakdown == nil {
		resp.StatusBreakdown = []struct {
			Count  int    `json:"count"`
			Status string `json:"status"`
		}{}
	}

	// Top products
	tpRows, err := s.pool.Query(ctx, `
		SELECT oi.name, oi.sku, COALESCE(sum(oi.quantity), 0)::int AS units, COALESCE(sum(oi.line_total_idr), 0)::bigint AS rev
		FROM order_items oi
		JOIN orders o ON o.id = oi.order_id
		WHERE o.status <> 'dibatalkan'
		GROUP BY oi.name, oi.sku
		ORDER BY rev DESC
		LIMIT 10
	`)
	if err == nil {
		for tpRows.Next() {
			var tp struct {
				Name       string `json:"name"`
				RevenueIdr int64  `json:"revenueIdr"`
				Sku        string `json:"sku"`
				Units      int    `json:"units"`
			}
			if tpRows.Scan(&tp.Name, &tp.Sku, &tp.Units, &tp.RevenueIdr) == nil {
				resp.TopProducts = append(resp.TopProducts, tp)
			}
		}
		tpRows.Close()
	}
	if resp.TopProducts == nil {
		resp.TopProducts = []struct {
			Name       string `json:"name"`
			RevenueIdr int64  `json:"revenueIdr"`
			Sku        string `json:"sku"`
			Units      int    `json:"units"`
		}{}
	}

	// Category sales
	catRows, err := s.pool.Query(ctx, `
		SELECT c.name, COALESCE(sum(oi.line_total_idr), 0)::bigint AS rev, COALESCE(sum(oi.quantity), 0)::int AS units
		FROM order_items oi
		JOIN orders o ON o.id = oi.order_id
		JOIN variants v ON v.id = oi.variant_id
		JOIN products p ON p.id = v.product_id
		JOIN categories c ON c.id = p.category_id
		WHERE o.status <> 'dibatalkan'
		GROUP BY c.name
		ORDER BY rev DESC
		LIMIT 8
	`)
	if err == nil {
		for catRows.Next() {
			var cs struct {
				Name       string `json:"name"`
				RevenueIdr int64  `json:"revenueIdr"`
				Units      int    `json:"units"`
			}
			if catRows.Scan(&cs.Name, &cs.RevenueIdr, &cs.Units) == nil {
				resp.CategorySales = append(resp.CategorySales, cs)
			}
		}
		catRows.Close()
	}
	if resp.CategorySales == nil {
		resp.CategorySales = []struct {
			Name       string `json:"name"`
			RevenueIdr int64  `json:"revenueIdr"`
			Units      int    `json:"units"`
		}{}
	}

	// Brand sales
	bRows, err := s.pool.Query(ctx, `
		SELECT oi.brand, COALESCE(sum(oi.line_total_idr), 0)::bigint AS rev, COALESCE(sum(oi.quantity), 0)::int AS units
		FROM order_items oi
		JOIN orders o ON o.id = oi.order_id
		WHERE o.status <> 'dibatalkan'
		GROUP BY oi.brand
		ORDER BY rev DESC
		LIMIT 8
	`)
	if err == nil {
		for bRows.Next() {
			var bs struct {
				Name       string `json:"name"`
				RevenueIdr int64  `json:"revenueIdr"`
				Units      int    `json:"units"`
			}
			if bRows.Scan(&bs.Name, &bs.RevenueIdr, &bs.Units) == nil {
				resp.BrandSales = append(resp.BrandSales, bs)
			}
		}
		bRows.Close()
	}
	if resp.BrandSales == nil {
		resp.BrandSales = []struct {
			Name       string `json:"name"`
			RevenueIdr int64  `json:"revenueIdr"`
			Units      int    `json:"units"`
		}{}
	}

	// Action lists
	resp.ActionLists.PendingConfirmation, _ = s.queryAdminOrderRows(ctx, "o.status = 'menunggu_konfirmasi'", nil, "o.created_at ASC", 8, 0)
	resp.ActionLists.PendingPayment, _ = s.queryAdminOrderRows(ctx, "o.status = 'menunggu_pembayaran'", nil, "o.created_at ASC", 8, 0)
	resp.ActionLists.ExpiringSoon, _ = s.queryAdminOrderRows(ctx, "o.status = 'menunggu_konfirmasi' AND o.created_at < now() - interval '42 hours'", nil, "o.created_at ASC", 8, 0)

	lowItems, _, _ := s.queryAdminInventoryItems(ctx, "", "low", canFinance, 10, 0)
	resp.ActionLists.LowStock = lowItems
	if resp.ActionLists.LowStock == nil {
		resp.ActionLists.LowStock = []openapi.AdminInventoryItem{}
	}

	return &resp, nil
}

func (s *Server) queryAdminInventoryItems(ctx context.Context, q, state string, canFinance bool, limit, offset int) ([]openapi.AdminInventoryItem, int, error) {
	clauses := []string{"p.deleted_at IS NULL", "v.is_active = true"}
	var args []any
	if q != "" {
		args = append(args, "%"+q+"%")
		clauses = append(clauses, fmt.Sprintf("(p.name ILIKE $%d OR v.sku ILIKE $%d)", len(args), len(args)))
	}
	switch state {
	case "low":
		clauses = append(clauses, "v.is_preorder = false AND v.stock > 0 AND v.stock <= v.low_stock_threshold")
	case "out":
		clauses = append(clauses, "v.is_preorder = false AND v.stock = 0")
	case "preorder":
		clauses = append(clauses, "v.is_preorder = true")
	}
	whereSQL := strings.Join(clauses, " AND ")

	var total int
	_ = s.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT count(*)::int FROM variants v JOIN products p ON p.id = v.product_id WHERE %s
	`, whereSQL), args...).Scan(&total)

	listArgs := append([]any{}, args...)
	listArgs = append(listArgs, limit, offset)
	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT v.id, p.id, p.name, p.slug, v.sku, v.price_idr, v.cost_idr, v.stock, v.low_stock_threshold, v.is_preorder
		FROM variants v
		JOIN products p ON p.id = v.product_id
		WHERE %s
		ORDER BY v.stock ASC, p.name ASC
		LIMIT $%d OFFSET $%d
	`, whereSQL, len(listArgs)-1, len(listArgs)), listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]openapi.AdminInventoryItem, 0)
	for rows.Next() {
		var it openapi.AdminInventoryItem
		var stock, thresh int32
		var cost *int64
		if err := rows.Scan(&it.VariantId, &it.ProductId, &it.ProductName, &it.ProductSlug, &it.Sku, &it.PriceIdr, &cost, &stock, &thresh, &it.IsPreorder); err == nil {
			it.Stock = int(stock)
			it.LowStockThreshold = int(thresh)
			it.StockStatus = inventory.ComputeStockStatus(stock, thresh, it.IsPreorder)
			if canFinance {
				it.CostIdr = cost
			}
			items = append(items, it)
		}
	}
	return items, total, nil
}
