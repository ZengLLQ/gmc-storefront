package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"gmc/api/internal/auth"
	"gmc/api/internal/gen/db"
	"gmc/api/internal/gen/openapi"
	"gmc/api/internal/platform"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func toOpenAPIUser(u db.User) openapi.User {
	mustChange := u.MustChangePassword
	return openapi.User{
		Id:                 u.ID,
		Email:              u.Email,
		Name:               u.Name,
		Phone:              u.Phone,
		Role:               openapi.UserRole(u.Role),
		MustChangePassword: &mustChange,
	}
}

// RegisterCustomer implements POST /v1/auth/register
func (s *Server) RegisterCustomer(w http.ResponseWriter, r *http.Request) {
	ip := platform.ClientIPFromContext(r.Context())
	if !s.registerLimiter.Allow(ip) {
		platform.WriteError(w, r, platform.NewAppError(http.StatusTooManyRequests, openapi.RateLimited, "Terlalu banyak percobaan pendaftaran, silakan coba lagi nanti"))
		return
	}

	var req openapi.RegisterRequest
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}

	fields := make(map[string]string)
	name := strings.TrimSpace(req.Name)
	if len(name) < 2 || len(name) > 100 || platform.HasControlChars(name) {
		fields["name"] = "Nama lengkap wajib diisi (2-100 karakter)"
	}
	email := strings.ToLower(strings.TrimSpace(string(req.Email)))
	if !strings.Contains(email, "@") || len(email) > 160 {
		fields["email"] = "Email tidak valid"
	}
	phone, err := platform.NormalizePhone(req.Phone)
	if err != nil {
		fields["phone"] = err.Error()
	}
	if err := platform.ValidatePasswordStrength(req.Password); err != nil {
		fields["password"] = err.Error()
	}
	if len(fields) > 0 {
		platform.WriteError(w, r, platform.NewValidationError("Data pendaftaran tidak valid", fields))
		return
	}

	if _, err := s.q.GetUserByEmail(r.Context(), email); err == nil {
		platform.WriteError(w, r, platform.NewAppError(http.StatusConflict, openapi.EmailTaken, "Email sudah terdaftar"))
		return
	}

	pwHash, err := platform.HashPassword(req.Password)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}

	user, err := s.q.CreateUser(r.Context(), db.CreateUserParams{
		ID:                 platform.NewUUID(),
		Email:              email,
		Name:               name,
		Phone:              phone,
		PasswordHash:       pwHash,
		Role:               "customer",
		IsActive:           true,
		MustChangePassword: false,
	})
	if err != nil {
		platform.WriteError(w, r, platform.NewAppError(http.StatusConflict, openapi.EmailTaken, "Email sudah terdaftar"))
		return
	}

	accessJWT, refreshRaw, err := s.authSvc.IssueSession(r.Context(), user, false, r.UserAgent(), ip)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	s.authSvc.SetAuthCookies(w, accessJWT, refreshRaw, false)
	platform.WritePrivateJSON(w, http.StatusCreated, openapi.AuthUserResponse{User: toOpenAPIUser(user)})
}

// LoginCustomer implements POST /v1/auth/login
func (s *Server) LoginCustomer(w http.ResponseWriter, r *http.Request) {
	ip := platform.ClientIPFromContext(r.Context())
	var req openapi.LoginRequest
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	email := strings.ToLower(strings.TrimSpace(string(req.Email)))
	if !s.loginLimiter.Allow(ip+":"+email) {
		platform.WriteError(w, r, platform.NewAppError(http.StatusTooManyRequests, openapi.RateLimited, "Terlalu banyak percobaan login, silakan tunggu 1 menit"))
		return
	}

	user, err := s.q.GetUserByEmail(r.Context(), email)
	if err != nil || !user.IsActive || !platform.VerifyPassword(req.Password, user.PasswordHash) {
		platform.WriteError(w, r, platform.NewAppError(http.StatusUnauthorized, openapi.InvalidCredentials, "Email atau password salah"))
		return
	}

	accessJWT, refreshRaw, err := s.authSvc.IssueSession(r.Context(), user, false, r.UserAgent(), ip)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	s.authSvc.SetAuthCookies(w, accessJWT, refreshRaw, false)
	platform.WritePrivateJSON(w, http.StatusOK, openapi.AuthUserResponse{User: toOpenAPIUser(user)})
}

// RefreshCustomerSession implements POST /v1/auth/refresh
func (s *Server) RefreshCustomerSession(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(auth.RefreshCookieName)
	if err != nil || strings.TrimSpace(c.Value) == "" {
		platform.WriteError(w, r, platform.NewAppError(http.StatusUnauthorized, openapi.Unauthorized, "Token refresh tidak ditemukan"))
		return
	}
	ip := platform.ClientIPFromContext(r.Context())
	accessJWT, refreshRaw, isAdmin, err := s.authSvc.RefreshSession(r.Context(), c.Value, r.UserAgent(), ip)
	if err != nil {
		s.authSvc.ClearAuthCookies(w)
		platform.WriteError(w, r, err)
		return
	}
	s.authSvc.SetAuthCookies(w, accessJWT, refreshRaw, isAdmin)
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusNoContent)
}

// LogoutCustomer implements POST /v1/auth/logout
func (s *Server) LogoutCustomer(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(auth.RefreshCookieName); err == nil && c.Value != "" {
		if sess, err := s.q.GetSessionByTokenHash(r.Context(), platform.HashToken(c.Value)); err == nil {
			_ = s.q.RevokeSessionByID(r.Context(), sess.ID)
		}
	}
	s.authSvc.ClearAuthCookies(w)
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusNoContent)
}

// ForgotPassword implements POST /v1/auth/forgot-password
func (s *Server) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	ip := platform.ClientIPFromContext(r.Context())
	if !s.forgotLimiter.Allow(ip) {
		platform.WriteError(w, r, platform.NewAppError(http.StatusTooManyRequests, openapi.RateLimited, "Terlalu banyak permintaan reset password"))
		return
	}
	var req struct {
		Email string `json:"email"`
	}
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	if s.cfg.AppEnv == "development" {
		slog.Info("password reset requested (dev stub)", "email", strings.ToLower(strings.TrimSpace(req.Email)))
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusNoContent)
}

// GetMe implements GET /v1/me
func (s *Server) GetMe(w http.ResponseWriter, r *http.Request) {
	_, user, err := s.authSvc.ExtractActor(r, false)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	platform.WritePrivateJSON(w, http.StatusOK, openapi.AuthUserResponse{User: toOpenAPIUser(*user)})
}

// UpdateMe implements PATCH /v1/me
func (s *Server) UpdateMe(w http.ResponseWriter, r *http.Request) {
	_, user, err := s.authSvc.ExtractActor(r, false)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	var req openapi.UpdateProfileRequest
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	fields := make(map[string]string)
	name := strings.TrimSpace(req.Name)
	if len(name) < 2 || len(name) > 100 || platform.HasControlChars(name) {
		fields["name"] = "Nama wajib diisi (2-100 karakter)"
	}
	email := strings.ToLower(strings.TrimSpace(string(req.Email)))
	if !strings.Contains(email, "@") {
		fields["email"] = "Email tidak valid"
	}
	phone, err := platform.NormalizePhone(req.Phone)
	if err != nil {
		fields["phone"] = err.Error()
	}
	if len(fields) > 0 {
		platform.WriteError(w, r, platform.NewValidationError("Data profil tidak valid", fields))
		return
	}

	if !strings.EqualFold(email, user.Email) {
		if existing, err := s.q.GetUserByEmail(r.Context(), email); err == nil && existing.ID != user.ID {
			platform.WriteError(w, r, platform.NewAppError(http.StatusConflict, openapi.EmailTaken, "Email sudah digunakan oleh akun lain"))
			return
		}
	}

	updated, err := s.q.UpdateUserProfile(r.Context(), db.UpdateUserProfileParams{
		ID:    user.ID,
		Name:  name,
		Email: email,
		Phone: phone,
	})
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	platform.WritePrivateJSON(w, http.StatusOK, openapi.AuthUserResponse{User: toOpenAPIUser(updated)})
}

// ChangeMyPassword implements PUT /v1/me/password
func (s *Server) ChangeMyPassword(w http.ResponseWriter, r *http.Request) {
	actor, user, err := s.authSvc.ExtractActor(r, false)
	if err != nil {
		platform.WriteError(w, r, err)
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
	newHash, err := platform.HashPassword(req.NewPassword)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	if err := s.q.UpdateUserPassword(r.Context(), db.UpdateUserPasswordParams{
		ID:           user.ID,
		PasswordHash: newHash,
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
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusNoContent)
}

func toOpenAPIAddress(a db.Address) openapi.Address {
	return openapi.Address{
		Id:            a.ID,
		Label:         a.Label,
		RecipientName: a.RecipientName,
		Phone:         a.Phone,
		Street:        a.Street,
		City:          a.City,
		Province:      a.Province,
		PostalCode:    a.PostalCode,
		IsDefault:     a.IsDefault,
	}
}

// ListMyAddresses implements GET /v1/me/addresses
func (s *Server) ListMyAddresses(w http.ResponseWriter, r *http.Request) {
	_, user, err := s.authSvc.ExtractActor(r, false)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	rows, err := s.q.ListUserAddresses(r.Context(), user.ID)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	items := make([]openapi.Address, 0, len(rows))
	for _, row := range rows {
		items = append(items, toOpenAPIAddress(row))
	}
	platform.WritePrivateJSON(w, http.StatusOK, map[string]any{"items": items})
}

// CreateMyAddress implements POST /v1/me/addresses
func (s *Server) CreateMyAddress(w http.ResponseWriter, r *http.Request) {
	_, user, err := s.authSvc.ExtractActor(r, false)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	cnt, _ := s.q.CountUserAddresses(r.Context(), user.ID)
	if cnt >= 10 {
		platform.WriteError(w, r, platform.NewValidationError("Maksimal 10 alamat tersimpan", nil))
		return
	}
	var req openapi.AddressInput
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	phone, err := platform.NormalizePhone(req.Phone)
	if err != nil {
		platform.WriteError(w, r, platform.NewValidationError(err.Error(), map[string]string{"phone": err.Error()}))
		return
	}

	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	qtx := s.q.WithTx(tx)

	isDef := req.IsDefault || cnt == 0
	if isDef {
		_ = qtx.ClearUserDefaultAddress(r.Context(), user.ID)
	}
	row, err := qtx.CreateAddress(r.Context(), db.CreateAddressParams{
		ID:            platform.NewUUID(),
		UserID:        user.ID,
		Label:         req.Label,
		RecipientName: strings.TrimSpace(req.RecipientName),
		Phone:         phone,
		Street:        strings.TrimSpace(req.Street),
		City:          strings.TrimSpace(req.City),
		Province:      strings.TrimSpace(req.Province),
		PostalCode:    strings.TrimSpace(req.PostalCode),
		IsDefault:     isDef,
	})
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	_ = tx.Commit(r.Context())
	platform.WritePrivateJSON(w, http.StatusCreated, toOpenAPIAddress(row))
}

// UpdateMyAddress implements PATCH /v1/me/addresses/{id}
func (s *Server) UpdateMyAddress(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	_, user, err := s.authSvc.ExtractActor(r, false)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	var req openapi.AddressInput
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	phone, err := platform.NormalizePhone(req.Phone)
	if err != nil {
		platform.WriteError(w, r, platform.NewValidationError(err.Error(), map[string]string{"phone": err.Error()}))
		return
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	qtx := s.q.WithTx(tx)

	if req.IsDefault {
		_ = qtx.ClearUserDefaultAddress(r.Context(), user.ID)
	}
	row, err := qtx.UpdateAddress(r.Context(), db.UpdateAddressParams{
		ID:            id,
		UserID:        user.ID,
		Label:         req.Label,
		RecipientName: strings.TrimSpace(req.RecipientName),
		Phone:         phone,
		Street:        strings.TrimSpace(req.Street),
		City:          strings.TrimSpace(req.City),
		Province:      strings.TrimSpace(req.Province),
		PostalCode:    strings.TrimSpace(req.PostalCode),
		IsDefault:     req.IsDefault,
	})
	if err != nil {
		platform.WriteError(w, r, platform.NewAppError(http.StatusNotFound, openapi.NotFound, "Alamat tidak ditemukan"))
		return
	}
	_ = tx.Commit(r.Context())
	platform.WritePrivateJSON(w, http.StatusOK, toOpenAPIAddress(row))
}

// DeleteMyAddress implements DELETE /v1/me/addresses/{id}
func (s *Server) DeleteMyAddress(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	_, user, err := s.authSvc.ExtractActor(r, false)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	n, err := s.q.DeleteAddress(r.Context(), db.DeleteAddressParams{ID: id, UserID: user.ID})
	if err != nil || n == 0 {
		platform.WriteError(w, r, platform.NewAppError(http.StatusNotFound, openapi.NotFound, "Alamat tidak ditemukan"))
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusNoContent)
}

// ListMyOrders implements GET /v1/me/orders
func (s *Server) ListMyOrders(w http.ResponseWriter, r *http.Request, params openapi.ListMyOrdersParams) {
	_, user, err := s.authSvc.ExtractActor(r, false)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	page := 1
	if params.Page != nil && *params.Page >= 1 {
		page = *params.Page
	}
	pageSize := 24
	if params.PageSize != nil && *params.PageSize >= 1 && *params.PageSize <= 60 {
		pageSize = *params.PageSize
	}

	clauses := []string{"(o.user_id = $1 OR lower(o.customer_email) = lower($2))"}
	args := []any{user.ID, user.Email}

	if params.Status != nil && *params.Status != "" {
		args = append(args, string(*params.Status))
		clauses = append(clauses, fmt.Sprintf("o.status = $%d", len(args)))
	}
	if params.Q != nil && strings.TrimSpace(*params.Q) != "" {
		args = append(args, strings.TrimSpace(*params.Q)+"%")
		clauses = append(clauses, fmt.Sprintf("o.order_no ILIKE $%d", len(args)))
	}

	whereSQL := strings.Join(clauses, " AND ")
	var total int
	_ = s.pool.QueryRow(r.Context(), "SELECT count(*)::int FROM orders o WHERE "+whereSQL, args...).Scan(&total)

	offset := (page - 1) * pageSize
	args = append(args, pageSize, offset)
	rows, err := s.pool.Query(r.Context(), fmt.Sprintf(`
		SELECT
			o.order_no, o.status, o.payment_status, o.created_at, o.total_idr,
			COALESCE((SELECT sum(oi.quantity)::int FROM order_items oi WHERE oi.order_id = o.id), 0) AS item_count,
			COALESCE((SELECT oi.name FROM order_items oi WHERE oi.order_id = o.id ORDER BY oi.id ASC LIMIT 1), '') AS first_name,
			(SELECT oi.image_url FROM order_items oi WHERE oi.order_id = o.id ORDER BY oi.id ASC LIMIT 1) AS first_img
		FROM orders o
		WHERE %s
		ORDER BY o.created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereSQL, len(args)-1, len(args)), args...)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	defer rows.Close()

	items := make([]openapi.OrderSummary, 0)
	for rows.Next() {
		var os openapi.OrderSummary
		var st, pst string
		if err := rows.Scan(&os.OrderNo, &st, &pst, &os.CreatedAt, &os.TotalIdr, &os.ItemCount, &os.FirstItemName, &os.FirstItemImageUrl); err == nil {
			os.Status = openapi.OrderStatus(st)
			os.PaymentStatus = openapi.PaymentStatus(pst)
			items = append(items, os)
		}
	}

	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}

	platform.WritePrivateJSON(w, http.StatusOK, openapi.OrderSummaryListResponse{
		Items:      items,
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
	})
}

// GetMyOrderByOrderNo implements GET /v1/me/orders/{orderNo}
func (s *Server) GetMyOrderByOrderNo(w http.ResponseWriter, r *http.Request, orderNo string) {
	_, user, err := s.authSvc.ExtractActor(r, false)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	ord, err := s.q.GetOrderByOrderNo(r.Context(), strings.TrimSpace(orderNo))
	if err != nil || (!ord.UserID.Valid || ord.UserID.UUID != user.ID) && !strings.EqualFold(ord.CustomerEmail, user.Email) {
		platform.WriteError(w, r, platform.NewAppError(http.StatusNotFound, openapi.NotFound, "Pesanan tidak ditemukan"))
		return
	}
	resp, err := s.buildPublicOrderResponse(r.Context(), ord)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	platform.WritePrivateJSON(w, http.StatusOK, resp)
}

// GetMyWishlist implements GET /v1/me/wishlist
func (s *Server) GetMyWishlist(w http.ResponseWriter, r *http.Request) {
	_, user, err := s.authSvc.ExtractActor(r, false)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	ids, err := s.q.ListUserWishlistProductIDs(r.Context(), user.ID)
	if err != nil || len(ids) == 0 {
		platform.WritePrivateJSON(w, http.StatusOK, map[string]any{"items": []openapi.ProductCard{}})
		return
	}

	res, err := s.queryPublicProducts(r.Context(), productFilter{page: 1, pageSize: 100})
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	wanted := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		wanted[id] = true
	}
	items := make([]openapi.ProductCard, 0)
	for _, p := range res.Items {
		if wanted[p.Id] {
			items = append(items, p)
		}
	}
	platform.WritePrivateJSON(w, http.StatusOK, map[string]any{"items": items})
}

// AddWishlistItem implements PUT /v1/me/wishlist/{productId}
func (s *Server) AddWishlistItem(w http.ResponseWriter, r *http.Request, productId openapi_types.UUID) {
	_, user, err := s.authSvc.ExtractActor(r, false)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	_ = s.q.AddWishlistItem(r.Context(), db.AddWishlistItemParams{
		UserID:    user.ID,
		ProductID: productId,
	})
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusNoContent)
}

// RemoveWishlistItem implements DELETE /v1/me/wishlist/{productId}
func (s *Server) RemoveWishlistItem(w http.ResponseWriter, r *http.Request, productId openapi_types.UUID) {
	_, user, err := s.authSvc.ExtractActor(r, false)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	_ = s.q.RemoveWishlistItem(r.Context(), db.RemoveWishlistItemParams{
		UserID:    user.ID,
		ProductID: productId,
	})
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusNoContent)
}

// SubmitContactMessage implements POST /v1/contact
func (s *Server) SubmitContactMessage(w http.ResponseWriter, r *http.Request) {
	ip := platform.ClientIPFromContext(r.Context())
	if !s.contactLimiter.Allow(ip) {
		platform.WriteError(w, r, platform.NewAppError(http.StatusTooManyRequests, openapi.RateLimited, "Terlalu banyak pengiriman pesan, silakan coba lagi nanti"))
		return
	}
	var req openapi.ContactRequest
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}

	// Honeypot check: if website field is filled, return 201 without storing
	if req.Website != nil && strings.TrimSpace(*req.Website) != "" {
		waURL := platform.BuildContactWhatsAppURL(s.cfg.WhatsAppNumber, s.cfg.StoreName, req.Name, string(req.Subject), req.Message)
		platform.WritePrivateJSON(w, http.StatusCreated, map[string]string{"whatsappUrl": waURL})
		return
	}

	fields := make(map[string]string)
	name := strings.TrimSpace(req.Name)
	if len(name) < 2 || len(name) > 100 {
		fields["name"] = "Nama wajib diisi"
	}
	email := strings.ToLower(strings.TrimSpace(string(req.Email)))
	if !strings.Contains(email, "@") {
		fields["email"] = "Email tidak valid"
	}
	phone, err := platform.NormalizePhone(req.Phone)
	if err != nil {
		fields["phone"] = err.Error()
	}
	msg := strings.TrimSpace(req.Message)
	if len(msg) < 10 || len(msg) > 2000 {
		fields["message"] = "Pesan harus terdiri dari 10 hingga 2000 karakter"
	}
	if len(fields) > 0 {
		platform.WriteError(w, r, platform.NewValidationError("Data formulir kontak belum sesuai", fields))
		return
	}

	_, err = s.q.CreateContactMessage(r.Context(), db.CreateContactMessageParams{
		ID:      platform.NewUUID(),
		Name:    name,
		Email:   email,
		Phone:   phone,
		Subject: string(req.Subject),
		Message: msg,
	})
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}

	waURL := platform.BuildContactWhatsAppURL(s.cfg.WhatsAppNumber, s.cfg.StoreName, name, string(req.Subject), msg)
	platform.WritePrivateJSON(w, http.StatusCreated, map[string]string{"whatsappUrl": waURL})
}

// SubscribeNewsletter implements POST /v1/newsletter
func (s *Server) SubscribeNewsletter(w http.ResponseWriter, r *http.Request) {
	ip := platform.ClientIPFromContext(r.Context())
	if !s.newsletterLimiter.Allow(ip) {
		platform.WriteError(w, r, platform.NewAppError(http.StatusTooManyRequests, openapi.RateLimited, "Terlalu banyak permintaan, silakan coba lagi nanti"))
		return
	}
	var req openapi.NewsletterRequest
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	email := strings.ToLower(strings.TrimSpace(string(req.Email)))
	if !strings.Contains(email, "@") || len(email) > 160 {
		platform.WriteError(w, r, platform.NewValidationError("Email tidak valid", map[string]string{"email": "Email tidak valid"}))
		return
	}
	_ = s.q.UpsertNewsletterSubscriber(r.Context(), db.UpsertNewsletterSubscriberParams{
		ID:    platform.NewUUID(),
		Email: email,
		Name:  req.Name,
	})
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusNoContent)
}
