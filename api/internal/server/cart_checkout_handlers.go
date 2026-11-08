package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"gmc/api/internal/gen/db"
	"gmc/api/internal/gen/openapi"
	"gmc/api/internal/inventory"
	"gmc/api/internal/orders"
	"gmc/api/internal/platform"
	"gmc/api/internal/promotions"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Server) resolveVariantLabel(ctx context.Context, optionValueIDs []uuid.UUID) string {
	if len(optionValueIDs) == 0 {
		return ""
	}
	rows, err := s.pool.Query(ctx, `
		SELECT label FROM product_option_values
		WHERE id = ANY($1)
		ORDER BY sort_order ASC
	`, optionValueIDs)
	if err != nil {
		return ""
	}
	defer rows.Close()
	var labels []string
	for rows.Next() {
		var l string
		if rows.Scan(&l) == nil && l != "" {
			labels = append(labels, l)
		}
	}
	return strings.Join(labels, " / ")
}

func (s *Server) evaluateCartLines(ctx context.Context, items []openapi.CartItemInput, voucherCode *string, userID uuid.NullUUID) (*openapi.CartValidationResponse, error) {
	lines := make([]openapi.CartLine, 0, len(items))
	var eligibleLines []promotions.EligibleLine
	var subtotal int64
	hasIssues := false

	for _, in := range items {
		qty := in.Quantity
		if qty < 1 {
			qty = 1
		}
		if qty > 99 {
			qty = 99
		}

		vr, err := s.q.GetVariantByID(ctx, in.VariantId)
		if err != nil || !vr.IsActive || !vr.ProductActive || vr.ProductDeletedAt != nil {
			issue := openapi.Unavailable
			hasIssues = true
			lines = append(lines, openapi.CartLine{
				VariantId:    in.VariantId,
				Sku:          "N/A",
				ProductSlug:  "",
				Name:         "Produk tidak tersedia",
				Brand:        "",
				VariantLabel: "",
				UnitPriceIdr: 0,
				Quantity:     qty,
				LineTotalIdr: 0,
				Stock:        0,
				StockStatus:  openapi.StockStatus("out_of_stock"),
				Issue:        &issue,
			})
			continue
		}

		var issuePtr *openapi.CartLineIssue
		if !vr.IsPreorder {
			if vr.Stock <= 0 {
				iss := openapi.CartLineIssueOutOfStock
				issuePtr = &iss
				hasIssues = true
			} else if int32(qty) > vr.Stock {
				iss := openapi.InsufficientStock
				issuePtr = &iss
				hasIssues = true
			}
		} else if qty > 5 {
			iss := openapi.InsufficientStock
			issuePtr = &iss
			hasIssues = true
		}

		lineTotal := vr.PriceIdr * int64(qty)
		subtotal += lineTotal
		imgURL := vr.ResolvedImageUrl
		label := s.resolveVariantLabel(ctx, vr.OptionValueIds)

		lines = append(lines, openapi.CartLine{
			VariantId:    vr.ID,
			Sku:          vr.Sku,
			ProductSlug:  vr.ProductSlug,
			Name:         vr.ProductName,
			Brand:        vr.BrandName,
			VariantLabel: label,
			ImageUrl:     &imgURL,
			UnitPriceIdr: vr.PriceIdr,
			Quantity:     qty,
			LineTotalIdr: lineTotal,
			Stock:        int(vr.Stock),
			StockStatus:  inventory.ComputeStockStatus(vr.Stock, vr.LowStockThreshold, vr.IsPreorder),
			Issue:        issuePtr,
		})

		eligibleLines = append(eligibleLines, promotions.EligibleLine{
			CategoryID:   vr.CategoryID,
			BrandID:      vr.BrandID,
			LineTotalIDR: lineTotal,
		})
	}

	var discountIDR int64
	var appliedVoucher *openapi.AppliedVoucherInfo
	var voucherIssue *string

	if voucherCode != nil && strings.TrimSpace(*voucherCode) != "" {
		code := strings.ToUpper(strings.TrimSpace(*voucherCode))
		vRow, err := s.q.GetVoucherByCode(ctx, code)
		if err != nil {
			msg := "Kode voucher tidak ditemukan"
			voucherIssue = &msg
		} else {
			var custUsed int64
			if userID.Valid {
				custUsed, _ = s.q.CountVoucherRedemptionsByUser(ctx, db.CountVoucherRedemptionsByUserParams{
					VoucherID: vRow.ID,
					UserID:    userID,
					Phone:     nil,
				})
			}
			eval := promotions.Evaluate(promotions.VoucherRule{
				ID:                vRow.ID,
				Code:              vRow.Code,
				Description:       vRow.Description,
				Kind:              vRow.Kind,
				Value:             vRow.Value,
				MaxDiscountIDR:    vRow.MaxDiscountIdr,
				MinSubtotalIDR:    vRow.MinSubtotalIdr,
				UsageLimit:        vRow.UsageLimit,
				PerUserLimit:      vRow.PerUserLimit,
				UsedCount:         vRow.UsedCount,
				StartsAt:          vRow.StartsAt,
				EndsAt:            vRow.EndsAt,
				IsActive:          vRow.IsActive,
				ScopeCategoryIDs:  vRow.ScopeCategoryIds,
				ScopeBrandIDs:     vRow.ScopeBrandIds,
				CustomerUsedCount: custUsed,
			}, eligibleLines, time.Now().UTC())

			if !eval.Valid {
				voucherIssue = &eval.Issue
			} else {
				discountIDR = eval.DiscountIDR
				appliedVoucher = &openapi.AppliedVoucherInfo{
					Code:        vRow.Code,
					Description: vRow.Description,
				}
			}
		}
	}

	totalIDR := subtotal - discountIDR
	if totalIDR < 0 {
		totalIDR = 0
	}

	return &openapi.CartValidationResponse{
		Lines:        lines,
		SubtotalIdr:  subtotal,
		DiscountIdr:  discountIDR,
		TotalIdr:     totalIDR,
		Voucher:      appliedVoucher,
		VoucherIssue: voucherIssue,
		HasIssues:    hasIssues,
	}, nil
}

// ValidateCart implements POST /v1/cart/validate
func (s *Server) ValidateCart(w http.ResponseWriter, r *http.Request) {
	var req openapi.CartValidateRequest
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	if len(req.Items) > 50 {
		platform.WriteError(w, r, platform.NewAppError(http.StatusBadRequest, openapi.BadRequest, "Maksimal 50 baris item di keranjang"))
		return
	}
	var userID uuid.NullUUID
	if _, u, err := s.authSvc.ExtractActor(r, false); err == nil && u != nil {
		userID = uuid.NullUUID{UUID: u.ID, Valid: true}
	}
	resp, err := s.evaluateCartLines(r.Context(), req.Items, req.VoucherCode, userID)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	platform.WritePrivateJSON(w, http.StatusOK, resp)
}

// GetMyCart implements GET /v1/me/cart
func (s *Server) GetMyCart(w http.ResponseWriter, r *http.Request) {
	_, user, err := s.authSvc.ExtractActor(r, false)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	rows, err := s.q.ListUserCartItems(r.Context(), user.ID)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	inputs := make([]openapi.CartItemInput, 0, len(rows))
	for _, row := range rows {
		inputs = append(inputs, openapi.CartItemInput{
			VariantId: row.VariantID,
			Quantity:  int(row.Quantity),
		})
	}
	resp, err := s.evaluateCartLines(r.Context(), inputs, nil, uuid.NullUUID{UUID: user.ID, Valid: true})
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	platform.WritePrivateJSON(w, http.StatusOK, resp)
}

// ReplaceMyCart implements PUT /v1/me/cart
func (s *Server) ReplaceMyCart(w http.ResponseWriter, r *http.Request) {
	_, user, err := s.authSvc.ExtractActor(r, false)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	var req openapi.CartValidateRequest
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	qtx := s.q.WithTx(tx)

	_ = qtx.UpsertUserCart(r.Context(), user.ID)
	_ = qtx.ClearUserCartItems(r.Context(), user.ID)
	for _, it := range req.Items {
		qty := it.Quantity
		if qty < 1 {
			qty = 1
		}
		if qty > 99 {
			qty = 99
		}
		_ = qtx.UpsertCartItem(r.Context(), db.UpsertCartItemParams{
			UserID:    user.ID,
			VariantID: it.VariantId,
			Quantity:  int32(qty),
		})
	}
	if err := tx.Commit(r.Context()); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	resp, err := s.evaluateCartLines(r.Context(), req.Items, req.VoucherCode, uuid.NullUUID{UUID: user.ID, Valid: true})
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	platform.WritePrivateJSON(w, http.StatusOK, resp)
}

// MergeMyCart implements POST /v1/me/cart/merge
func (s *Server) MergeMyCart(w http.ResponseWriter, r *http.Request) {
	_, user, err := s.authSvc.ExtractActor(r, false)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	var req openapi.CartValidateRequest
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}

	existing, _ := s.q.ListUserCartItems(r.Context(), user.ID)
	mergedMap := make(map[uuid.UUID]int)
	var orderIDs []uuid.UUID
	for _, e := range existing {
		mergedMap[e.VariantID] = int(e.Quantity)
		orderIDs = append(orderIDs, e.VariantID)
	}
	for _, g := range req.Items {
		if _, exists := mergedMap[g.VariantId]; !exists {
			orderIDs = append(orderIDs, g.VariantId)
		}
		mergedMap[g.VariantId] += g.Quantity
	}

	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	qtx := s.q.WithTx(tx)

	_ = qtx.UpsertUserCart(r.Context(), user.ID)
	_ = qtx.ClearUserCartItems(r.Context(), user.ID)

	var finalInputs []openapi.CartItemInput
	for _, vid := range orderIDs {
		qty := mergedMap[vid]
		if qty < 1 {
			qty = 1
		}
		vr, err := qtx.GetVariantByID(r.Context(), vid)
		if err == nil && vr.IsActive && vr.ProductActive && vr.ProductDeletedAt == nil {
			maxAllowed := int(vr.Stock)
			if vr.IsPreorder {
				maxAllowed = 5
			}
			if maxAllowed > 99 {
				maxAllowed = 99
			}
			if maxAllowed > 0 && qty > maxAllowed {
				qty = maxAllowed
			}
			if maxAllowed > 0 {
				_ = qtx.UpsertCartItem(r.Context(), db.UpsertCartItemParams{
					UserID:    user.ID,
					VariantID: vid,
					Quantity:  int32(qty),
				})
			}
		}
		finalInputs = append(finalInputs, openapi.CartItemInput{
			VariantId: vid,
			Quantity:  qty,
		})
	}
	_ = tx.Commit(r.Context())

	resp, err := s.evaluateCartLines(r.Context(), finalInputs, req.VoucherCode, uuid.NullUUID{UUID: user.ID, Valid: true})
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	platform.WritePrivateJSON(w, http.StatusOK, resp)
}

// CreateCheckoutOrder implements POST /v1/checkout
func (s *Server) CreateCheckoutOrder(w http.ResponseWriter, r *http.Request, params openapi.CreateCheckoutOrderParams) {
	ip := platform.ClientIPFromContext(r.Context())
	if !s.checkoutLimiter.Allow(ip) {
		platform.WriteError(w, r, platform.NewAppError(http.StatusTooManyRequests, openapi.RateLimited, "Terlalu banyak permintaan checkout, coba lagi sebentar"))
		return
	}

	rawBody, err := io.ReadAll(http.MaxBytesReader(nil, r.Body, 1<<20))
	if err != nil {
		platform.WriteError(w, r, platform.NewAppError(http.StatusBadRequest, openapi.BadRequest, "Request body terlalu besar atau tidak valid"))
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(rawBody))

	reqHashArr := sha256.Sum256(rawBody)
	reqHash := reqHashArr[:]
	idemKey := params.IdempotencyKey

	// Check existing Idempotency-Key
	if existing, err := s.q.GetIdempotencyKey(r.Context(), idemKey); err == nil {
		if !platform.ConstantTimeHashEqual(existing.RequestHash, reqHash) {
			platform.WriteError(w, r, platform.NewAppError(http.StatusConflict, openapi.IdempotencyConflict, "Idempotency-Key yang sama digunakan dengan data pesanan yang berbeda"))
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "private, no-store")
		w.WriteHeader(int(existing.StatusCode))
		_, _ = w.Write(existing.Response)
		return
	}

	// Check if store is open
	siteInfo, err := s.loadPublicSite(r.Context())
	if err == nil && !siteInfo.Status.IsOpen {
		msg := siteInfo.Status.Message
		if msg == "" {
			msg = "Mohon maaf, toko sedang tutup sementara."
		}
		platform.WriteError(w, r, platform.NewAppError(http.StatusServiceUnavailable, openapi.StoreClosed, msg))
		return
	}

	var req openapi.CheckoutRequest
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}

	// Validate inputs
	fields := make(map[string]string)
	custName := strings.TrimSpace(req.Customer.Name)
	if len(custName) < 2 || len(custName) > 120 || platform.HasControlChars(custName) {
		fields["customer.name"] = "Nama lengkap wajib diisi (2-120 karakter)"
	}
	custEmail := strings.ToLower(strings.TrimSpace(string(req.Customer.Email)))
	if !strings.Contains(custEmail, "@") || len(custEmail) > 160 {
		fields["customer.email"] = "Alamat email tidak valid"
	}
	custPhone, phoneErr := platform.NormalizePhone(req.Customer.Phone)
	if phoneErr != nil {
		fields["customer.phone"] = phoneErr.Error()
	}
	if len(req.Items) == 0 || len(req.Items) > 50 {
		fields["items"] = "Keranjang belanja harus berisi antara 1 hingga 50 item"
	}

	var shippingAddrJSON []byte
	var cityForWA string
	if req.DeliveryMethod == openapi.Delivery {
		if req.ShippingAddress == nil {
			fields["shippingAddress"] = "Alamat pengiriman wajib diisi untuk metode Kirim"
		} else {
			recPhone, err := platform.NormalizePhone(req.ShippingAddress.Phone)
			if err != nil {
				fields["shippingAddress.phone"] = err.Error()
			} else {
				req.ShippingAddress.Phone = recPhone
			}
			if strings.TrimSpace(req.ShippingAddress.RecipientName) == "" {
				fields["shippingAddress.recipientName"] = "Nama penerima wajib diisi"
			}
			if strings.TrimSpace(req.ShippingAddress.Street) == "" {
				fields["shippingAddress.street"] = "Alamat lengkap wajib diisi"
			}
			if strings.TrimSpace(req.ShippingAddress.City) == "" {
				fields["shippingAddress.city"] = "Kota/Kabupaten wajib diisi"
			}
			if strings.TrimSpace(req.ShippingAddress.Province) == "" {
				fields["shippingAddress.province"] = "Provinsi wajib diisi"
			}
			if strings.TrimSpace(req.ShippingAddress.PostalCode) == "" {
				fields["shippingAddress.postalCode"] = "Kode pos wajib diisi"
			}
			cityForWA = strings.TrimSpace(req.ShippingAddress.City)
			shippingAddrJSON, _ = json.Marshal(req.ShippingAddress)
		}
	}

	if len(fields) > 0 {
		platform.WriteError(w, r, platform.NewValidationError("Data checkout belum lengkap atau tidak valid", fields))
		return
	}

	var userID uuid.NullUUID
	if _, u, err := s.authSvc.ExtractActor(r, false); err == nil && u != nil {
		userID = uuid.NullUUID{UUID: u.ID, Valid: true}
	} else if siteInfo != nil && !siteInfo.Checkout.AllowGuest {
		platform.WriteError(w, r, platform.NewAppError(http.StatusUnauthorized, openapi.Unauthorized, "Silakan masuk untuk melanjutkan checkout"))
		return
	}

	// Consolidate duplicate variant lines and sort by VariantId to guarantee deadlock-free lock ordering
	qtyByVariant := make(map[uuid.UUID]int)
	var sortedVarIDs []uuid.UUID
	for _, it := range req.Items {
		if it.Quantity < 1 || it.Quantity > 99 {
			platform.WriteError(w, r, platform.NewValidationError("Jumlah item tidak valid", map[string]string{"items": "Jumlah per item harus antara 1 dan 99"}))
			return
		}
		if _, exists := qtyByVariant[it.VariantId]; !exists {
			sortedVarIDs = append(sortedVarIDs, it.VariantId)
		}
		qtyByVariant[it.VariantId] += it.Quantity
	}
	sort.Slice(sortedVarIDs, func(i, j int) bool {
		return sortedVarIDs[i].String() < sortedVarIDs[j].String()
	})

	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	qtx := s.q.WithTx(tx)

	orderID := platform.NewUUID()
	var subtotalIDR int64
	var outOfStockSKUs []string
	type preparedItem struct {
		variantID    uuid.UUID
		productSlug  string
		sku          string
		name         string
		brand        string
		variantLabel string
		imageURL     string
		unitPriceIDR int64
		costIDR      *int64
		quantity     int
		lineTotalIDR int64
		categoryID   uuid.UUID
		brandID      uuid.UUID
		isPreorder   bool
	}
	var prepared []preparedItem

	for _, vid := range sortedVarIDs {
		qty := qtyByVariant[vid]
		vr, err := qtx.LockVariantForUpdate(r.Context(), vid)
		if err != nil || !vr.IsActive || !vr.ProductActive || vr.ProductDeletedAt != nil {
			outOfStockSKUs = append(outOfStockSKUs, vid.String())
			continue
		}
		if vr.IsPreorder {
			if qty > 5 {
				outOfStockSKUs = append(outOfStockSKUs, vr.Sku)
				continue
			}
		} else {
			if vr.Stock < int32(qty) {
				outOfStockSKUs = append(outOfStockSKUs, vr.Sku)
				continue
			}
		}
		lineTotal := vr.PriceIdr * int64(qty)
		subtotalIDR += lineTotal
		label := s.resolveVariantLabel(r.Context(), vr.OptionValueIds)
		prepared = append(prepared, preparedItem{
			variantID:    vr.ID,
			productSlug:  vr.ProductSlug,
			sku:          vr.Sku,
			name:         vr.ProductName,
			brand:        vr.BrandName,
			variantLabel: label,
			imageURL:     vr.ResolvedImageUrl,
			unitPriceIDR: vr.PriceIdr,
			costIDR:      vr.CostIdr,
			quantity:     qty,
			lineTotalIDR: lineTotal,
			categoryID:   vr.CategoryID,
			brandID:      vr.BrandID,
			isPreorder:   vr.IsPreorder,
		})
	}

	if len(outOfStockSKUs) > 0 {
		platform.WriteError(w, r, platform.NewAppError(
			http.StatusConflict,
			openapi.ErrorCode("out_of_stock"),
			fmt.Sprintf("Stok tidak mencukupi untuk SKU: %s", strings.Join(outOfStockSKUs, ", ")),
		))
		return
	}

	if req.ExpectedSubtotalIdr != nil && *req.ExpectedSubtotalIdr != subtotalIDR {
		platform.WriteError(w, r, platform.NewAppError(
			http.StatusConflict,
			openapi.PriceChanged,
			"Harga produk di keranjang telah diperbarui. Silakan periksa kembali total belanja Anda.",
		))
		return
	}

	// Evaluate & lock voucher if present
	var discountIDR int64
	var voucherCodePtr *string
	var lockedVoucher *db.Voucher
	if req.VoucherCode != nil && strings.TrimSpace(*req.VoucherCode) != "" {
		vCode := strings.ToUpper(strings.TrimSpace(*req.VoucherCode))
		vRow, err := qtx.LockVoucherByCode(r.Context(), vCode)
		if err != nil {
			platform.WriteError(w, r, platform.NewValidationError("Kode voucher tidak valid", map[string]string{
				"voucherCode": "Kode voucher tidak ditemukan",
			}))
			return
		}
		custUsed, _ := qtx.CountVoucherRedemptionsByUser(r.Context(), db.CountVoucherRedemptionsByUserParams{
			VoucherID: vRow.ID,
			UserID:    userID,
			Phone:     &custPhone,
		})
		var elig []promotions.EligibleLine
		for _, p := range prepared {
			elig = append(elig, promotions.EligibleLine{
				CategoryID:   p.categoryID,
				BrandID:      p.brandID,
				LineTotalIDR: p.lineTotalIDR,
			})
		}
		eval := promotions.Evaluate(promotions.VoucherRule{
			ID:                vRow.ID,
			Code:              vRow.Code,
			Description:       vRow.Description,
			Kind:              vRow.Kind,
			Value:             vRow.Value,
			MaxDiscountIDR:    vRow.MaxDiscountIdr,
			MinSubtotalIDR:    vRow.MinSubtotalIdr,
			UsageLimit:        vRow.UsageLimit,
			PerUserLimit:      vRow.PerUserLimit,
			UsedCount:         vRow.UsedCount,
			StartsAt:          vRow.StartsAt,
			EndsAt:            vRow.EndsAt,
			IsActive:          vRow.IsActive,
			ScopeCategoryIDs:  vRow.ScopeCategoryIds,
			ScopeBrandIDs:     vRow.ScopeBrandIds,
			CustomerUsedCount: custUsed,
		}, elig, time.Now().UTC())
		if !eval.Valid {
			platform.WriteError(w, r, platform.NewValidationError(eval.Issue, map[string]string{
				"voucherCode": eval.Issue,
			}))
			return
		}
		discountIDR = eval.DiscountIDR
		vCodeCopy := vRow.Code
		voucherCodePtr = &vCodeCopy
		lockedVoucher = &vRow
	}

	// Decrement stock through the single-source-of-truth Inventory Ledger
	for _, p := range prepared {
		if !p.isPreorder {
			if _, err := s.invSvc.Adjust(r.Context(), qtx, inventory.AdjustParams{
				VariantID: p.variantID,
				Delta:     -int32(p.quantity),
				Reason:    "order",
				RefType:   "order",
				RefID:     uuid.NullUUID{UUID: orderID, Valid: true},
				ActorID:   userID,
				Note:      "Checkout pesanan pelanggan",
			}); err != nil {
				platform.WriteError(w, r, err)
				return
			}
		}
	}

	now := time.Now().UTC()
	localDay := time.Date(now.In(s.cfg.StoreTZ).Year(), now.In(s.cfg.StoreTZ).Month(), now.In(s.cfg.StoreTZ).Day(), 0, 0, 0, 0, time.UTC)
	var seq int32
	if err := tx.QueryRow(r.Context(), `
		INSERT INTO order_daily_seq (day, last_seq) VALUES ($1, 1)
		ON CONFLICT (day) DO UPDATE SET last_seq = order_daily_seq.last_seq + 1
		RETURNING last_seq
	`, localDay).Scan(&seq); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	orderNo := orders.FormatOrderNo(now, s.cfg.StoreTZ, seq)

	viewTokenPlain, viewTokenHash, err := platform.GenerateOpaqueToken()
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}

	totalIDR := subtotalIDR - discountIDR
	var notePtr *string
	if req.Note != nil && strings.TrimSpace(*req.Note) != "" {
		trimmed := strings.TrimSpace(*req.Note)
		notePtr = &trimmed
	}

	orderRow, err := qtx.CreateOrder(r.Context(), db.CreateOrderParams{
		ID:              orderID,
		OrderNo:         orderNo,
		UserID:          userID,
		CustomerName:    custName,
		CustomerPhone:   custPhone,
		CustomerEmail:   custEmail,
		DeliveryMethod:  string(req.DeliveryMethod),
		ShippingAddress: shippingAddrJSON,
		PaymentMethod:   string(req.PaymentMethod),
		Note:            notePtr,
		SubtotalIdr:     subtotalIDR,
		DiscountIdr:     discountIDR,
		ShippingIdr:     0,
		TotalIdr:        totalIDR,
		Status:          "menunggu_konfirmasi",
		PaymentStatus:   "unpaid",
		PaidAt:          nil,
		VoucherCode:     voucherCodePtr,
		Source:          "web",
		InternalNote:    nil,
		ViewTokenHash:   viewTokenHash,
		IdempotencyKey:  uuid.NullUUID{UUID: idemKey, Valid: true},
		RequestHash:     reqHash,
	})
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}

	var apiItems []openapi.OrderItem
	var waLines []platform.WhatsAppOrderLine
	for _, p := range prepared {
		imgCopy := p.imageURL
		_, err := qtx.CreateOrderItem(r.Context(), db.CreateOrderItemParams{
			ID:           platform.NewUUID(),
			OrderID:      orderID,
			VariantID:    uuid.NullUUID{UUID: p.variantID, Valid: true},
			ProductSlug:  p.productSlug,
			Sku:          p.sku,
			Name:         p.name,
			Brand:        p.brand,
			VariantLabel: p.variantLabel,
			ImageUrl:     &imgCopy,
			UnitPriceIdr: p.unitPriceIDR,
			CostIdr:      p.costIDR,
			Quantity:     int32(p.quantity),
			LineTotalIdr: p.lineTotalIDR,
		})
		if err != nil {
			platform.WriteError(w, r, err)
			return
		}
		varIDCopy := p.variantID
		apiItems = append(apiItems, openapi.OrderItem{
			VariantId:    &varIDCopy,
			ProductSlug:  p.productSlug,
			Sku:          p.sku,
			Name:         p.name,
			Brand:        p.brand,
			VariantLabel: p.variantLabel,
			ImageUrl:     &imgCopy,
			UnitPriceIdr: p.unitPriceIDR,
			Quantity:     p.quantity,
			LineTotalIdr: p.lineTotalIDR,
		})
		waLines = append(waLines, platform.WhatsAppOrderLine{
			Name:         p.name,
			VariantLabel: p.variantLabel,
			Quantity:     p.quantity,
			LineTotalIDR: p.lineTotalIDR,
		})
	}

	initNote := "Pesanan dibuat dan menunggu konfirmasi admin via WhatsApp"
	histRow, err := qtx.CreateOrderStatusHistory(r.Context(), db.CreateOrderStatusHistoryParams{
		ID:      platform.NewUUID(),
		OrderID: orderID,
		Status:  "menunggu_konfirmasi",
		Note:    &initNote,
	})
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}

	if lockedVoucher != nil {
		_ = qtx.IncrementVoucherUsedCount(r.Context(), lockedVoucher.ID)
		_ = qtx.CreateVoucherRedemption(r.Context(), db.CreateVoucherRedemptionParams{
			ID:          platform.NewUUID(),
			VoucherID:   lockedVoucher.ID,
			OrderID:     orderID,
			UserID:      userID,
			Phone:       &custPhone,
			DiscountIdr: discountIDR,
		})
	}

	if userID.Valid {
		_ = qtx.ClearUserCartItems(r.Context(), userID.UUID)
	}

	waURL := platform.BuildOrderWhatsAppURL(platform.WhatsAppOrderParams{
		WhatsAppNumber: s.cfg.WhatsAppNumber,
		StoreName:      s.cfg.StoreName,
		OrderNo:        orderNo,
		CustomerName:   custName,
		DeliveryMethod: string(req.DeliveryMethod),
		City:           cityForWA,
		Items:          waLines,
		TotalIDR:       totalIDR,
	})

	var viewTokenResp *string
	if !userID.Valid {
		viewTokenResp = &viewTokenPlain
	}

	checkoutResp := openapi.CheckoutResponse{
		Order: openapi.Order{
			Id:              orderRow.ID,
			OrderNo:         orderRow.OrderNo,
			Status:          openapi.OrderStatus(orderRow.Status),
			PaymentStatus:   openapi.PaymentStatus(orderRow.PaymentStatus),
			PaidAt:          orderRow.PaidAt,
			CreatedAt:       orderRow.CreatedAt,
			CustomerName:    orderRow.CustomerName,
			CustomerPhone:   orderRow.CustomerPhone,
			CustomerEmail:   orderRow.CustomerEmail,
			DeliveryMethod:  openapi.DeliveryMethod(orderRow.DeliveryMethod),
			PaymentMethod:   openapi.PaymentMethod(orderRow.PaymentMethod),
			ShippingAddress: req.ShippingAddress,
			Note:            orderRow.Note,
			VoucherCode:     orderRow.VoucherCode,
			Items:           apiItems,
			SubtotalIdr:     orderRow.SubtotalIdr,
			DiscountIdr:     orderRow.DiscountIdr,
			ShippingIdr:     orderRow.ShippingIdr,
			TotalIdr:        orderRow.TotalIdr,
			History: []openapi.OrderHistoryEntry{
				{
					Status:    openapi.OrderStatus(histRow.Status),
					Note:      histRow.Note,
					CreatedAt: histRow.CreatedAt,
				},
			},
			WhatsappUrl: waURL,
		},
		ViewToken: viewTokenResp,
	}

	respBytes, _ := json.Marshal(checkoutResp)
	_ = qtx.SaveIdempotencyKey(r.Context(), db.SaveIdempotencyKeyParams{
		Key:         idemKey,
		RequestHash: reqHash,
		Response:    respBytes,
		StatusCode:  http.StatusCreated,
	})

	if err := tx.Commit(r.Context()); err != nil {
		platform.WriteError(w, r, err)
		return
	}

	s.InvalidateCatalogCache()
	platform.WritePrivateJSON(w, http.StatusCreated, checkoutResp)
}

func (s *Server) buildPublicOrderResponse(ctx context.Context, ord db.Order) (*openapi.Order, error) {
	itemRows, err := s.q.ListOrderItemsByOrderID(ctx, ord.ID)
	if err != nil {
		return nil, err
	}
	histRows, err := s.q.ListOrderStatusHistoryByOrderID(ctx, ord.ID)
	if err != nil {
		return nil, err
	}

	items := make([]openapi.OrderItem, 0, len(itemRows))
	waLines := make([]platform.WhatsAppOrderLine, 0, len(itemRows))
	for _, it := range itemRows {
		var vidPtr *uuid.UUID
		if it.VariantID.Valid {
			v := it.VariantID.UUID
			vidPtr = &v
		}
		items = append(items, openapi.OrderItem{
			VariantId:    vidPtr,
			ProductSlug:  it.ProductSlug,
			Sku:          it.Sku,
			Name:         it.Name,
			Brand:        it.Brand,
			VariantLabel: it.VariantLabel,
			ImageUrl:     it.ImageUrl,
			UnitPriceIdr: it.UnitPriceIdr,
			Quantity:     int(it.Quantity),
			LineTotalIdr: it.LineTotalIdr,
		})
		waLines = append(waLines, platform.WhatsAppOrderLine{
			Name:         it.Name,
			VariantLabel: it.VariantLabel,
			Quantity:     int(it.Quantity),
			LineTotalIDR: it.LineTotalIdr,
		})
	}

	history := make([]openapi.OrderHistoryEntry, 0, len(histRows))
	for _, h := range histRows {
		history = append(history, openapi.OrderHistoryEntry{
			Status:    openapi.OrderStatus(h.Status),
			Note:      h.Note,
			CreatedAt: h.CreatedAt,
		})
	}

	var shipAddr *openapi.ShippingAddress
	var city string
	if len(ord.ShippingAddress) > 0 {
		var sa openapi.ShippingAddress
		if json.Unmarshal(ord.ShippingAddress, &sa) == nil {
			shipAddr = &sa
			city = sa.City
		}
	}

	waURL := platform.BuildOrderWhatsAppURL(platform.WhatsAppOrderParams{
		WhatsAppNumber: s.cfg.WhatsAppNumber,
		StoreName:      s.cfg.StoreName,
		OrderNo:        ord.OrderNo,
		CustomerName:   ord.CustomerName,
		DeliveryMethod: ord.DeliveryMethod,
		City:           city,
		Items:          waLines,
		TotalIDR:       ord.TotalIdr,
	})

	return &openapi.Order{
		Id:              ord.ID,
		OrderNo:         ord.OrderNo,
		Status:          openapi.OrderStatus(ord.Status),
		PaymentStatus:   openapi.PaymentStatus(ord.PaymentStatus),
		PaidAt:          ord.PaidAt,
		CreatedAt:       ord.CreatedAt,
		CustomerName:    ord.CustomerName,
		CustomerPhone:   ord.CustomerPhone,
		CustomerEmail:   ord.CustomerEmail,
		DeliveryMethod:  openapi.DeliveryMethod(ord.DeliveryMethod),
		PaymentMethod:   openapi.PaymentMethod(ord.PaymentMethod),
		ShippingAddress: shipAddr,
		Note:            ord.Note,
		VoucherCode:     ord.VoucherCode,
		Courier:         ord.Courier,
		TrackingNo:      ord.TrackingNo,
		Items:           items,
		SubtotalIdr:     ord.SubtotalIdr,
		DiscountIdr:     ord.DiscountIdr,
		ShippingIdr:     ord.ShippingIdr,
		TotalIdr:        ord.TotalIdr,
		History:         history,
		WhatsappUrl:     waURL,
	}, nil
}

// GetOrderByOrderNo implements GET /v1/orders/{orderNo}
func (s *Server) GetOrderByOrderNo(w http.ResponseWriter, r *http.Request, orderNo string, params openapi.GetOrderByOrderNoParams) {
	ord, err := s.q.GetOrderByOrderNo(r.Context(), strings.TrimSpace(orderNo))
	if err != nil {
		if err == pgx.ErrNoRows {
			platform.WriteError(w, r, platform.NewAppError(http.StatusNotFound, openapi.NotFound, "Pesanan tidak ditemukan"))
			return
		}
		platform.WriteError(w, r, err)
		return
	}

	authorized := false
	if params.Token != nil && strings.TrimSpace(*params.Token) != "" {
		tokenHash := platform.HashToken(strings.TrimSpace(*params.Token))
		if platform.ConstantTimeHashEqual(tokenHash, ord.ViewTokenHash) {
			authorized = true
		}
	}
	if !authorized {
		if _, user, err := s.authSvc.ExtractActor(r, false); err == nil && user != nil {
			if (ord.UserID.Valid && ord.UserID.UUID == user.ID) || strings.EqualFold(ord.CustomerEmail, user.Email) {
				authorized = true
			}
		}
	}

	if !authorized {
		// Return 404 (not 403) to prevent order-number probing
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
