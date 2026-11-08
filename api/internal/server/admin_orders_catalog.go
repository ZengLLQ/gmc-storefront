package server

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"gmc/api/internal/admin/rbac"
	"gmc/api/internal/audit"
	"gmc/api/internal/gen/db"
	"gmc/api/internal/gen/openapi"
	"gmc/api/internal/inventory"
	"gmc/api/internal/orders"
	"gmc/api/internal/platform"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func (s *Server) queryAdminOrderRows(ctx context.Context, whereSQL string, args []any, orderBy string, limit, offset int) ([]openapi.AdminOrderRow, error) {
	if whereSQL == "" {
		whereSQL = "true"
	}
	if orderBy == "" {
		orderBy = "o.created_at DESC"
	}
	listArgs := append([]any{}, args...)
	listArgs = append(listArgs, limit, offset)

	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT
			o.id, o.order_no, o.status, o.payment_status, o.source, o.created_at,
			o.customer_name, o.customer_phone, o.customer_email,
			o.delivery_method, o.payment_method, o.total_idr, o.voucher_code,
			COALESCE((SELECT sum(oi.quantity)::int FROM order_items oi WHERE oi.order_id = o.id), 0) AS item_count,
			COALESCE((SELECT string_agg(oi.name || ' x' || oi.quantity, ', ' ORDER BY oi.id ASC) FROM order_items oi WHERE oi.order_id = o.id), '') AS items_summary
		FROM orders o
		WHERE %s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, whereSQL, orderBy, len(listArgs)-1, len(listArgs)), listArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]openapi.AdminOrderRow, 0)
	for rows.Next() {
		var row openapi.AdminOrderRow
		var st, pst, dm, pm string
		if err := rows.Scan(
			&row.Id, &row.OrderNo, &st, &pst, &row.Source, &row.CreatedAt,
			&row.CustomerName, &row.CustomerPhone, &row.CustomerEmail,
			&dm, &pm, &row.TotalIdr, &row.VoucherCode,
			&row.ItemCount, &row.ItemsSummary,
		); err == nil {
			row.Status = openapi.OrderStatus(st)
			row.PaymentStatus = openapi.PaymentStatus(pst)
			row.DeliveryMethod = openapi.DeliveryMethod(dm)
			row.PaymentMethod = openapi.PaymentMethod(pm)
			row.WhatsappUrl = fmt.Sprintf("https://wa.me/%s", platform.NormalizePhoneOrKeep(row.CustomerPhone))
			items = append(items, row)
		}
	}
	return items, nil
}

// AdminListOrders implements GET /v1/admin/orders
func (s *Server) AdminListOrders(w http.ResponseWriter, r *http.Request, params openapi.AdminListOrdersParams) {
	_, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermOrdersRead)
	if !ok {
		return
	}
	page := 1
	if params.Page != nil && *params.Page >= 1 {
		page = *params.Page
	}
	pageSize := 24
	if params.PageSize != nil && *params.PageSize >= 1 && *params.PageSize <= 100 {
		pageSize = *params.PageSize
	}

	clauses := []string{"true"}
	var args []any
	addArg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if params.Q != nil && strings.TrimSpace(*params.Q) != "" {
		ph := addArg("%" + strings.TrimSpace(*params.Q) + "%")
		clauses = append(clauses, fmt.Sprintf("(o.order_no ILIKE %s OR o.customer_name ILIKE %s OR o.customer_phone ILIKE %s OR o.customer_email ILIKE %s OR EXISTS (SELECT 1 FROM order_items oi WHERE oi.order_id = o.id AND oi.sku ILIKE %s))", ph, ph, ph, ph, ph))
	}
	if params.Status != nil && len(*params.Status) > 0 {
		var sts []string
		for _, st := range *params.Status {
			sts = append(sts, string(st))
		}
		ph := addArg(sts)
		clauses = append(clauses, fmt.Sprintf("o.status = ANY(%s)", ph))
	}
	if params.PaymentStatus != nil && *params.PaymentStatus != "" {
		ph := addArg(string(*params.PaymentStatus))
		clauses = append(clauses, fmt.Sprintf("o.payment_status = %s", ph))
	}
	if params.DeliveryMethod != nil && *params.DeliveryMethod != "" {
		ph := addArg(string(*params.DeliveryMethod))
		clauses = append(clauses, fmt.Sprintf("o.delivery_method = %s", ph))
	}
	if params.PaymentMethod != nil && *params.PaymentMethod != "" {
		ph := addArg(string(*params.PaymentMethod))
		clauses = append(clauses, fmt.Sprintf("o.payment_method = %s", ph))
	}
	if params.Source != nil && *params.Source != "" {
		ph := addArg(string(*params.Source))
		clauses = append(clauses, fmt.Sprintf("o.source = %s", ph))
	}
	if params.HasVoucher != nil {
		if *params.HasVoucher {
			clauses = append(clauses, "o.voucher_code IS NOT NULL")
		} else {
			clauses = append(clauses, "o.voucher_code IS NULL")
		}
	}

	whereSQL := strings.Join(clauses, " AND ")
	var total int
	_ = s.pool.QueryRow(r.Context(), "SELECT count(*)::int FROM orders o WHERE "+whereSQL, args...).Scan(&total)

	offset := (page - 1) * pageSize
	items, err := s.queryAdminOrderRows(r.Context(), whereSQL, args, "o.created_at DESC", pageSize, offset)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}

	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}

	platform.WritePrivateJSON(w, http.StatusOK, openapi.AdminOrderListResponse{
		Items:      items,
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
	})
}

// AdminExportOrdersCsv implements GET /v1/admin/orders/export.csv
func (s *Server) AdminExportOrdersCsv(w http.ResponseWriter, r *http.Request, params openapi.AdminExportOrdersCsvParams) {
	_, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermOrdersRead)
	if !ok {
		return
	}
	rows, err := s.queryAdminOrderRows(r.Context(), "true", nil, "o.created_at DESC", 50000, 0)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	var csvRows [][]string
	for _, r := range rows {
		csvRows = append(csvRows, []string{
			r.OrderNo,
			r.CreatedAt.In(s.cfg.StoreTZ).Format(time.RFC3339),
			r.CustomerName,
			r.CustomerPhone,
			r.CustomerEmail,
			string(r.Status),
			string(r.PaymentStatus),
			string(r.DeliveryMethod),
			string(r.PaymentMethod),
			strconv.FormatInt(r.TotalIdr, 10),
			r.ItemsSummary,
		})
	}
	var delim *string
	if params.Delimiter != nil {
		d := string(*params.Delimiter)
		delim = &d
	}
	writeCSVDownload(w, r, "gmc-orders.csv", delim, []string{
		"No. Pesanan", "Tanggal", "Pelanggan", "No. WhatsApp", "Email",
		"Status Pesanan", "Status Pembayaran", "Pengiriman", "Pembayaran", "Total (IDR)", "Ringkasan Item",
	}, csvRows)
}

func (s *Server) buildAdminOrderResponse(ctx context.Context, ord db.Order, canFinance bool) (*openapi.AdminOrder, error) {
	itemRows, err := s.q.ListOrderItemsByOrderID(ctx, ord.ID)
	if err != nil {
		return nil, err
	}
	histRows, err := s.q.ListOrderStatusHistoryByOrderID(ctx, ord.ID)
	if err != nil {
		return nil, err
	}

	items := make([]openapi.AdminOrderItem, 0, len(itemRows))
	for _, it := range itemRows {
		var vidPtr *uuid.UUID
		if it.VariantID.Valid {
			v := it.VariantID.UUID
			vidPtr = &v
		}
		var costPtr *int64
		if canFinance {
			costPtr = it.CostIdr
		}
		items = append(items, openapi.AdminOrderItem{
			VariantId:    vidPtr,
			ProductSlug:  it.ProductSlug,
			Sku:          it.Sku,
			Name:         it.Name,
			Brand:        it.Brand,
			VariantLabel: it.VariantLabel,
			ImageUrl:     it.ImageUrl,
			UnitPriceIdr: it.UnitPriceIdr,
			CostIdr:      costPtr,
			Quantity:     int(it.Quantity),
			LineTotalIdr: it.LineTotalIdr,
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
	if len(ord.ShippingAddress) > 0 {
		var sa openapi.ShippingAddress
		if json.Unmarshal(ord.ShippingAddress, &sa) == nil {
			shipAddr = &sa
		}
	}

	var userIDPtr *uuid.UUID
	if ord.UserID.Valid {
		u := ord.UserID.UUID
		userIDPtr = &u
	}

	waURL := fmt.Sprintf("https://wa.me/%s", platform.NormalizePhoneOrKeep(ord.CustomerPhone))

	return &openapi.AdminOrder{
		Id:              ord.ID,
		OrderNo:         ord.OrderNo,
		UserId:          userIDPtr,
		Status:          openapi.OrderStatus(ord.Status),
		PaymentStatus:   openapi.PaymentStatus(ord.PaymentStatus),
		PaidAt:          ord.PaidAt,
		Source:          ord.Source,
		Version:         int(ord.Version),
		CreatedAt:       ord.CreatedAt,
		CustomerName:    ord.CustomerName,
		CustomerPhone:   ord.CustomerPhone,
		CustomerEmail:   ord.CustomerEmail,
		DeliveryMethod:  openapi.DeliveryMethod(ord.DeliveryMethod),
		PaymentMethod:   openapi.PaymentMethod(ord.PaymentMethod),
		ShippingAddress: shipAddr,
		Note:            ord.Note,
		InternalNote:    ord.InternalNote,
		CancelReason:    ord.CancelReason,
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

// AdminGetOrder implements GET /v1/admin/orders/{orderNo}
func (s *Server) AdminGetOrder(w http.ResponseWriter, r *http.Request, orderNo string) {
	_, user, ok := s.authSvc.RequirePermission(w, r, rbac.PermOrdersRead)
	if !ok {
		return
	}
	ord, err := s.q.GetOrderByOrderNo(r.Context(), strings.TrimSpace(orderNo))
	if err != nil {
		platform.WriteError(w, r, platform.NewAppError(http.StatusNotFound, openapi.NotFound, "Pesanan tidak ditemukan"))
		return
	}
	resp, err := s.buildAdminOrderResponse(r.Context(), ord, rbac.HasPermission(user.Role, rbac.PermFinanceView))
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	platform.WritePrivateJSON(w, http.StatusOK, resp)
}

// AdminCreateManualOrder implements POST /v1/admin/orders
func (s *Server) AdminCreateManualOrder(w http.ResponseWriter, r *http.Request, params openapi.AdminCreateManualOrderParams) {
	_, user, ok := s.authSvc.RequirePermission(w, r, rbac.PermOrdersWrite)
	if !ok {
		return
	}
	var req openapi.AdminManualOrderRequest
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}

	custPhone, err := platform.NormalizePhone(req.Customer.Phone)
	if err != nil {
		platform.WriteError(w, r, platform.NewValidationError(err.Error(), map[string]string{"customer.phone": err.Error()}))
		return
	}
	custName := strings.TrimSpace(req.Customer.Name)
	custEmail := strings.ToLower(strings.TrimSpace(string(req.Customer.Email)))

	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	qtx := s.q.WithTx(tx)

	orderID := platform.NewUUID()
	var subtotalIDR int64
	type manItem struct {
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
		isPreorder   bool
	}
	var prepared []manItem

	for _, it := range req.Items {
		if it.Quantity < 1 {
			platform.WriteError(w, r, platform.NewValidationError("Jumlah item minimal 1", nil))
			return
		}
		vr, err := qtx.LockVariantForUpdate(r.Context(), it.VariantId)
		if err != nil {
			platform.WriteError(w, r, platform.NewAppError(http.StatusNotFound, openapi.NotFound, "Varian tidak ditemukan"))
			return
		}
		unitPrice := vr.PriceIdr
		if it.OverridePriceIdr != nil && *it.OverridePriceIdr >= 0 {
			if it.OverrideReason == nil || strings.TrimSpace(*it.OverrideReason) == "" {
				platform.WriteError(w, r, platform.NewValidationError("Alasan perubahan harga manual wajib diisi", map[string]string{"overrideReason": "Wajib diisi"}))
				return
			}
			unitPrice = *it.OverridePriceIdr
		}
		if !vr.IsPreorder {
			if _, err := s.invSvc.Adjust(r.Context(), qtx, inventory.AdjustParams{
				VariantID: vr.ID,
				Delta:     -int32(it.Quantity),
				Reason:    "order",
				RefType:   "order",
				RefID:     uuid.NullUUID{UUID: orderID, Valid: true},
				ActorID:   uuid.NullUUID{UUID: user.ID, Valid: true},
				Note:      "Pesanan manual admin",
			}); err != nil {
				platform.WriteError(w, r, err)
				return
			}
		}
		lineTotal := unitPrice * int64(it.Quantity)
		subtotalIDR += lineTotal
		label := s.resolveVariantLabel(r.Context(), vr.OptionValueIds)
		prepared = append(prepared, manItem{
			variantID:    vr.ID,
			productSlug:  vr.ProductSlug,
			sku:          vr.Sku,
			name:         vr.ProductName,
			brand:        vr.BrandName,
			variantLabel: label,
			imageURL:     vr.ResolvedImageUrl,
			unitPriceIDR: unitPrice,
			costIDR:      vr.CostIdr,
			quantity:     it.Quantity,
			lineTotalIDR: lineTotal,
			isPreorder:   vr.IsPreorder,
		})
	}

	var discountIDR int64
	if req.ManualDiscountIdr != nil && *req.ManualDiscountIdr > 0 {
		discountIDR = *req.ManualDiscountIdr
		if discountIDR > subtotalIDR {
			discountIDR = subtotalIDR
		}
	}
	var shippingIDR int64
	if req.ShippingIdr != nil && *req.ShippingIdr >= 0 {
		shippingIDR = *req.ShippingIdr
	}
	totalIDR := subtotalIDR - discountIDR + shippingIDR

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

	initStatus := "menunggu_konfirmasi"
	if req.InitialStatus != nil && *req.InitialStatus != "" {
		initStatus = string(*req.InitialStatus)
	}
	payStatus := "unpaid"
	var paidAt *time.Time
	if req.MarkPaid != nil && *req.MarkPaid {
		payStatus = "paid"
		paidAt = &now
	}

	var shipAddrBytes []byte
	if req.ShippingAddress != nil {
		shipAddrBytes, _ = json.Marshal(req.ShippingAddress)
	}
	_, viewTokenHash, _ := platform.GenerateOpaqueToken()

	var userID uuid.NullUUID
	if req.UserId != nil {
		userID = uuid.NullUUID{UUID: *req.UserId, Valid: true}
	}

	ordRow, err := qtx.CreateOrder(r.Context(), db.CreateOrderParams{
		ID:              orderID,
		OrderNo:         orderNo,
		UserID:          userID,
		CustomerName:    custName,
		CustomerPhone:   custPhone,
		CustomerEmail:   custEmail,
		DeliveryMethod:  string(req.DeliveryMethod),
		ShippingAddress: shipAddrBytes,
		PaymentMethod:   string(req.PaymentMethod),
		Note:            req.Note,
		SubtotalIdr:     subtotalIDR,
		DiscountIdr:     discountIDR,
		ShippingIdr:     shippingIDR,
		TotalIdr:        totalIDR,
		Status:          initStatus,
		PaymentStatus:   payStatus,
		PaidAt:          paidAt,
		VoucherCode:     req.VoucherCode,
		Source:          "manual",
		InternalNote:    req.InternalNote,
		ViewTokenHash:   viewTokenHash,
	})
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}

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
	}

	histNote := "Pesanan manual dibuat oleh admin"
	_, _ = qtx.CreateOrderStatusHistory(r.Context(), db.CreateOrderStatusHistoryParams{
		ID:      platform.NewUUID(),
		OrderID: orderID,
		Status:  initStatus,
		Note:    &histNote,
	})

	_ = audit.Record(r.Context(), qtx, audit.Entry{
		ActorID:    uuid.NullUUID{UUID: user.ID, Valid: true},
		ActorEmail: user.Email,
		ActorRole:  user.Role,
		Action:     "order.create_manual",
		EntityType: "order",
		EntityID:   orderNo,
		Summary:    fmt.Sprintf("Membuat pesanan manual %s untuk %s (Rp %s)", orderNo, custName, platform.FormatIDRNumber(totalIDR)),
		After:      map[string]any{"orderNo": orderNo, "totalIdr": totalIDR, "status": initStatus},
	})

	if err := tx.Commit(r.Context()); err != nil {
		platform.WriteError(w, r, err)
		return
	}

	s.InvalidateCatalogCache()
	resp, err := s.buildAdminOrderResponse(r.Context(), ordRow, rbac.HasPermission(user.Role, rbac.PermFinanceView))
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	platform.WritePrivateJSON(w, http.StatusCreated, resp)
}

func (s *Server) transitionOrderInternal(ctx context.Context, orderNo string, nextStatus string, note *string, cancelReason *string, actor *db.User) (*db.Order, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)

	ord, err := qtx.LockOrderByOrderNo(ctx, strings.TrimSpace(orderNo))
	if err != nil {
		return nil, platform.NewAppError(http.StatusNotFound, openapi.NotFound, "Pesanan tidak ditemukan")
	}

	if !orders.CanTransition(ord.Status, nextStatus) {
		return nil, platform.NewAppError(
			http.StatusConflict,
			openapi.Conflict,
			fmt.Sprintf("Transisi status tidak valid dari %s ke %s", ord.Status, nextStatus),
		)
	}

	var actorID uuid.NullUUID
	actorEmail := "system"
	actorRole := "system"
	if actor != nil {
		actorID = uuid.NullUUID{UUID: actor.ID, Valid: true}
		actorEmail = actor.Email
		actorRole = actor.Role
	}

	if nextStatus == "dibatalkan" {
		if (cancelReason == nil || strings.TrimSpace(*cancelReason) == "") && (note == nil || strings.TrimSpace(*note) == "") {
			return nil, platform.NewValidationError("Alasan pembatalan wajib diisi", map[string]string{"cancelReason": "Alasan pembatalan wajib diisi"})
		}
		if cancelReason == nil && note != nil {
			cancelReason = note
		}
		// Restore stock through ledger
		items, err := qtx.ListOrderItemsByOrderID(ctx, ord.ID)
		if err != nil {
			return nil, err
		}
		for _, it := range items {
			if it.VariantID.Valid {
				vr, err := qtx.GetVariantByID(ctx, it.VariantID.UUID)
				if err == nil && !vr.IsPreorder {
					if _, err := s.invSvc.Adjust(ctx, qtx, inventory.AdjustParams{
						VariantID: it.VariantID.UUID,
						Delta:     it.Quantity,
						Reason:    "order_cancel",
						RefType:   "order",
						RefID:     uuid.NullUUID{UUID: ord.ID, Valid: true},
						ActorID:   actorID,
						Note:      fmt.Sprintf("Pembatalan pesanan %s", ord.OrderNo),
					}); err != nil {
						return nil, err
					}
				}
			}
		}
		// Release voucher redemption if any
		if ord.VoucherCode != nil && *ord.VoucherCode != "" {
			_ = qtx.DecrementVoucherUsedCount(ctx, *ord.VoucherCode)
			_ = qtx.DeleteVoucherRedemptionByOrder(ctx, ord.ID)
		}
	}

	updated, err := qtx.UpdateOrderStatus(ctx, db.UpdateOrderStatusParams{
		ID:           ord.ID,
		Status:       nextStatus,
		CancelReason: cancelReason,
	})
	if err != nil {
		return nil, err
	}

	histNote := note
	if histNote == nil && cancelReason != nil {
		histNote = cancelReason
	}
	_, _ = qtx.CreateOrderStatusHistory(ctx, db.CreateOrderStatusHistoryParams{
		ID:      platform.NewUUID(),
		OrderID: ord.ID,
		Status:  nextStatus,
		Note:    histNote,
	})

	_ = audit.Record(ctx, qtx, audit.Entry{
		ActorID:    actorID,
		ActorEmail: actorEmail,
		ActorRole:  actorRole,
		Action:     "order.status_change",
		EntityType: "order",
		EntityID:   ord.OrderNo,
		Summary:    fmt.Sprintf("Mengubah status pesanan %s dari %s menjadi %s", ord.OrderNo, ord.Status, nextStatus),
		Before:     map[string]any{"status": ord.Status},
		After:      map[string]any{"status": nextStatus, "cancelReason": cancelReason},
	})

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	s.InvalidateCatalogCache()
	return &updated, nil
}

// AdminTransitionOrderStatus implements POST /v1/admin/orders/{orderNo}/status
func (s *Server) AdminTransitionOrderStatus(w http.ResponseWriter, r *http.Request, orderNo string) {
	_, user, ok := s.authSvc.RequirePermission(w, r, rbac.PermOrdersWrite)
	if !ok {
		return
	}
	var req openapi.AdminOrderStatusRequest
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	updated, err := s.transitionOrderInternal(r.Context(), orderNo, string(req.Status), req.Note, req.CancelReason, user)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	resp, err := s.buildAdminOrderResponse(r.Context(), *updated, rbac.HasPermission(user.Role, rbac.PermFinanceView))
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	platform.WritePrivateJSON(w, http.StatusOK, resp)
}

// AdminBulkUpdateOrderStatus implements POST /v1/admin/orders/bulk-status
func (s *Server) AdminBulkUpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	_, user, ok := s.authSvc.RequirePermission(w, r, rbac.PermOrdersWrite)
	if !ok {
		return
	}
	var req openapi.AdminBulkOrderStatusRequest
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	resp := openapi.AdminBulkOrderStatusResponse{
		Updated: []string{},
		Failures: []struct {
			Message string `json:"message"`
			OrderNo string `json:"orderNo"`
		}{},
	}
	for _, no := range req.OrderNos {
		if _, err := s.transitionOrderInternal(r.Context(), no, string(req.Status), req.Note, req.Note, user); err != nil {
			resp.Failures = append(resp.Failures, struct {
				Message string `json:"message"`
				OrderNo string `json:"orderNo"`
			}{
				OrderNo: no,
				Message: err.Error(),
			})
		} else {
			resp.Updated = append(resp.Updated, no)
		}
	}
	platform.WritePrivateJSON(w, http.StatusOK, resp)
}

// AdminUpdateOrderMeta implements PATCH /v1/admin/orders/{orderNo}
func (s *Server) AdminUpdateOrderMeta(w http.ResponseWriter, r *http.Request, orderNo string) {
	_, user, ok := s.authSvc.RequirePermission(w, r, rbac.PermOrdersWrite)
	if !ok {
		return
	}
	var req openapi.AdminOrderPatchRequest
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

	ord, err := qtx.LockOrderByOrderNo(r.Context(), strings.TrimSpace(orderNo))
	if err != nil {
		platform.WriteError(w, r, platform.NewAppError(http.StatusNotFound, openapi.NotFound, "Pesanan tidak ditemukan"))
		return
	}

	shipping := ord.ShippingIdr
	if req.ShippingIdr != nil && *req.ShippingIdr >= 0 {
		shipping = *req.ShippingIdr
	}
	courier := ord.Courier
	if req.Courier != nil {
		courier = req.Courier
	}
	tracking := ord.TrackingNo
	if req.TrackingNo != nil {
		tracking = req.TrackingNo
	}
	payStatus := ord.PaymentStatus
	paidAt := ord.PaidAt
	if req.PaymentStatus != nil && *req.PaymentStatus != "" {
		payStatus = string(*req.PaymentStatus)
		if payStatus == "paid" && paidAt == nil {
			now := time.Now().UTC()
			paidAt = &now
		}
	}
	internalNote := ord.InternalNote
	if req.InternalNote != nil {
		internalNote = req.InternalNote
	}

	updated, err := qtx.UpdateOrderMeta(r.Context(), db.UpdateOrderMetaParams{
		ID:            ord.ID,
		ShippingIdr:   shipping,
		Courier:       courier,
		TrackingNo:    tracking,
		PaymentStatus: payStatus,
		PaidAt:        paidAt,
		InternalNote:  internalNote,
	})
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}

	_ = audit.Record(r.Context(), qtx, audit.Entry{
		ActorID:    uuid.NullUUID{UUID: user.ID, Valid: true},
		ActorEmail: user.Email,
		ActorRole:  user.Role,
		Action:     "order.update_meta",
		EntityType: "order",
		EntityID:   ord.OrderNo,
		Summary:    fmt.Sprintf("Memperbarui ongkir/resi/pembayaran pesanan %s", ord.OrderNo),
		Before:     map[string]any{"shippingIdr": ord.ShippingIdr, "paymentStatus": ord.PaymentStatus, "trackingNo": ord.TrackingNo},
		After:      map[string]any{"shippingIdr": updated.ShippingIdr, "paymentStatus": updated.PaymentStatus, "trackingNo": updated.TrackingNo},
	})

	if err := tx.Commit(r.Context()); err != nil {
		platform.WriteError(w, r, err)
		return
	}

	resp, err := s.buildAdminOrderResponse(r.Context(), updated, rbac.HasPermission(user.Role, rbac.PermFinanceView))
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	platform.WritePrivateJSON(w, http.StatusOK, resp)
}

// AdminUpdateOrderItems implements PUT /v1/admin/orders/{orderNo}/items
func (s *Server) AdminUpdateOrderItems(w http.ResponseWriter, r *http.Request, orderNo string) {
	_, user, ok := s.authSvc.RequirePermission(w, r, rbac.PermOrdersWrite)
	if !ok {
		return
	}
	var req openapi.AdminOrderItemsUpdateRequest
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

	ord, err := qtx.LockOrderByOrderNo(r.Context(), strings.TrimSpace(orderNo))
	if err != nil {
		platform.WriteError(w, r, platform.NewAppError(http.StatusNotFound, openapi.NotFound, "Pesanan tidak ditemukan"))
		return
	}
	if int(ord.Version) != req.Version {
		platform.WriteError(w, r, platform.NewAppError(http.StatusConflict, openapi.Conflict, "Data sudah diubah oleh orang lain. Muat ulang halaman."))
		return
	}
	if ord.Status != "menunggu_konfirmasi" && ord.Status != "menunggu_pembayaran" {
		platform.WriteError(w, r, platform.NewAppError(http.StatusConflict, openapi.Conflict, "Item pesanan hanya dapat diubah saat menunggu konfirmasi atau menunggu pembayaran"))
		return
	}

	oldItems, err := qtx.ListOrderItemsByOrderID(r.Context(), ord.ID)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}

	// Restore old items stock first
	for _, old := range oldItems {
		if old.VariantID.Valid {
			vr, err := qtx.GetVariantByID(r.Context(), old.VariantID.UUID)
			if err == nil && !vr.IsPreorder {
				if _, err := s.invSvc.Adjust(r.Context(), qtx, inventory.AdjustParams{
					VariantID: old.VariantID.UUID,
					Delta:     old.Quantity,
					Reason:    "order_edit",
					RefType:   "order",
					RefID:     uuid.NullUUID{UUID: ord.ID, Valid: true},
					ActorID:   uuid.NullUUID{UUID: user.ID, Valid: true},
					Note:      fmt.Sprintf("Revisi item pesanan %s (kembalikan stok lama)", ord.OrderNo),
				}); err != nil {
					platform.WriteError(w, r, err)
					return
				}
			}
		}
	}

	if err := qtx.DeleteOrderItemsByOrderID(r.Context(), ord.ID); err != nil {
		platform.WriteError(w, r, err)
		return
	}

	var newSubtotal int64
	for _, in := range req.Items {
		vr, err := qtx.LockVariantForUpdate(r.Context(), in.VariantId)
		if err != nil {
			platform.WriteError(w, r, platform.NewAppError(http.StatusNotFound, openapi.NotFound, "Varian tidak ditemukan"))
			return
		}
		if !vr.IsPreorder {
			if _, err := s.invSvc.Adjust(r.Context(), qtx, inventory.AdjustParams{
				VariantID: vr.ID,
				Delta:     -int32(in.Quantity),
				Reason:    "order_edit",
				RefType:   "order",
				RefID:     uuid.NullUUID{UUID: ord.ID, Valid: true},
				ActorID:   uuid.NullUUID{UUID: user.ID, Valid: true},
				Note:      fmt.Sprintf("Revisi item pesanan %s (ambil stok baru)", ord.OrderNo),
			}); err != nil {
				platform.WriteError(w, r, err)
				return
			}
		}
		lineTotal := vr.PriceIdr * int64(in.Quantity)
		newSubtotal += lineTotal
		imgCopy := vr.ResolvedImageUrl
		label := s.resolveVariantLabel(r.Context(), vr.OptionValueIds)
		_, err = qtx.CreateOrderItem(r.Context(), db.CreateOrderItemParams{
			ID:           platform.NewUUID(),
			OrderID:      ord.ID,
			VariantID:    uuid.NullUUID{UUID: vr.ID, Valid: true},
			ProductSlug:  vr.ProductSlug,
			Sku:          vr.Sku,
			Name:         vr.ProductName,
			Brand:        vr.BrandName,
			VariantLabel: label,
			ImageUrl:     &imgCopy,
			UnitPriceIdr: vr.PriceIdr,
			CostIdr:      vr.CostIdr,
			Quantity:     int32(in.Quantity),
			LineTotalIdr: lineTotal,
		})
		if err != nil {
			platform.WriteError(w, r, err)
			return
		}
	}

	discount := ord.DiscountIdr
	if discount > newSubtotal {
		discount = newSubtotal
	}
	newTotal := newSubtotal - discount + ord.ShippingIdr

	_, err = tx.Exec(r.Context(), `
		UPDATE orders
		SET subtotal_idr = $2, discount_idr = $3, total_idr = $4, version = version + 1, updated_at = now()
		WHERE id = $1
	`, ord.ID, newSubtotal, discount, newTotal)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}

	_ = audit.Record(r.Context(), qtx, audit.Entry{
		ActorID:    uuid.NullUUID{UUID: user.ID, Valid: true},
		ActorEmail: user.Email,
		ActorRole:  user.Role,
		Action:     "order.update_items",
		EntityType: "order",
		EntityID:   ord.OrderNo,
		Summary:    fmt.Sprintf("Mengubah daftar item pesanan %s (Total baru: Rp %s)", ord.OrderNo, platform.FormatIDRNumber(newTotal)),
	})

	if err := tx.Commit(r.Context()); err != nil {
		platform.WriteError(w, r, err)
		return
	}

	s.InvalidateCatalogCache()
	updated, _ := s.q.GetOrderByOrderNo(r.Context(), ord.OrderNo)
	resp, err := s.buildAdminOrderResponse(r.Context(), updated, rbac.HasPermission(user.Role, rbac.PermFinanceView))
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	platform.WritePrivateJSON(w, http.StatusOK, resp)
}

// AdminGetOrderWhatsapp implements GET /v1/admin/orders/{orderNo}/whatsapp
func (s *Server) AdminGetOrderWhatsapp(w http.ResponseWriter, r *http.Request, orderNo string, params openapi.AdminGetOrderWhatsappParams) {
	_, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermOrdersRead)
	if !ok {
		return
	}
	ord, err := s.q.GetOrderByOrderNo(r.Context(), strings.TrimSpace(orderNo))
	if err != nil {
		platform.WriteError(w, r, platform.NewAppError(http.StatusNotFound, openapi.NotFound, "Pesanan tidak ditemukan"))
		return
	}
	items, _ := s.q.ListOrderItemsByOrderID(r.Context(), ord.ID)
	var itemLines []string
	for i, it := range items {
		itemLines = append(itemLines, fmt.Sprintf("%d. %s x%d (Rp %s)", i+1, it.Name, it.Quantity, platform.FormatIDRNumber(it.LineTotalIdr)))
	}

	templates := map[string]string{
		"confirm": "Halo {{customerName}}, terima kasih telah memesan di Global Mandiri Computer.\n\nNo. Pesanan: {{orderNo}}\n{{items}}\nTotal: Rp {{total}}\n\nStok produk Anda tersedia. Silakan konfirmasi untuk melanjutkan ke pembayaran.",
		"payment": "Halo {{customerName}}, berikut informasi pembayaran untuk pesanan {{orderNo}} sebesar Rp {{total}}:\n\n{{bankAccounts}}\n\nMohon kirimkan bukti transfer melalui WhatsApp ini. Terima kasih.",
		"shipped": "Halo {{customerName}}, pesanan {{orderNo}} Anda telah dikirim!\n\nKurir: {{courier}}\nNo. Resi: {{trackingNo}}\n\nTerima kasih telah berbelanja di Global Mandiri Computer.",
		"ready":   "Halo {{customerName}}, pesanan {{orderNo}} Anda sudah siap diambil di:\n{{storeAddress}}\n\nSilakan tunjukkan nomor pesanan saat pengambilan.",
		"thanks":  "Terima kasih {{customerName}} telah berbelanja di Global Mandiri Computer! Semoga perangkat Anda berfungsi optimal. Hubungi kami kapan saja jika membutuhkan bantuan.",
		"custom":  "Halo {{customerName}}, terkait pesanan {{orderNo}} di Global Mandiri Computer:",
	}

	tpl := templates[string(params.Template)]
	if tpl == "" {
		tpl = templates["confirm"]
	}

	courier := "-"
	if ord.Courier != nil {
		courier = *ord.Courier
	}
	tracking := "-"
	if ord.TrackingNo != nil {
		tracking = *ord.TrackingNo
	}

	urlStr, textStr := platform.RenderAdminOrderWhatsApp(ord.CustomerPhone, tpl, map[string]string{
		"customerName": ord.CustomerName,
		"orderNo":      ord.OrderNo,
		"total":        platform.FormatIDRNumber(ord.TotalIdr),
		"items":        strings.Join(itemLines, "\n"),
		"bankAccounts": "BCA 0881234567 a/n Global Mandiri Computer",
		"courier":      courier,
		"trackingNo":   tracking,
		"storeAddress": "ITC Mega Grosir Lt. 2 Blok B/7, Surabaya",
	})

	platform.WritePrivateJSON(w, http.StatusOK, map[string]string{
		"url":  urlStr,
		"text": textStr,
	})
}

// ============================================================================
// ADMIN PRODUCTS, CATEGORIES, BRANDS, UPLOADS
// ============================================================================

// AdminListProducts implements GET /v1/admin/products
func (s *Server) AdminListProducts(w http.ResponseWriter, r *http.Request, params openapi.AdminListProductsParams) {
	_, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermProductsRead)
	if !ok {
		return
	}
	page := 1
	if params.Page != nil && *params.Page >= 1 {
		page = *params.Page
	}
	pageSize := 24
	if params.PageSize != nil && *params.PageSize >= 1 && *params.PageSize <= 100 {
		pageSize = *params.PageSize
	}

	clauses := []string{"true"}
	var args []any
	addArg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if params.Status != nil {
		switch *params.Status {
		case openapi.AdminListProductsParamsStatusArchived:
			clauses = append(clauses, "p.deleted_at IS NOT NULL")
		case openapi.AdminListProductsParamsStatusActive:
			clauses = append(clauses, "p.deleted_at IS NULL AND p.is_active = true")
		case openapi.AdminListProductsParamsStatusInactive:
			clauses = append(clauses, "p.deleted_at IS NULL AND p.is_active = false")
		}
	} else {
		clauses = append(clauses, "p.deleted_at IS NULL")
	}

	if params.Q != nil && strings.TrimSpace(*params.Q) != "" {
		ph := addArg("%" + strings.TrimSpace(*params.Q) + "%")
		clauses = append(clauses, fmt.Sprintf("(p.name ILIKE %s OR p.slug ILIKE %s OR EXISTS (SELECT 1 FROM variants v WHERE v.product_id = p.id AND (v.sku ILIKE %s OR COALESCE(v.barcode,'') ILIKE %s)))", ph, ph, ph, ph))
	}
	if params.Category != nil && strings.TrimSpace(*params.Category) != "" {
		ph := addArg(strings.TrimSpace(*params.Category))
		clauses = append(clauses, fmt.Sprintf("c.slug = %s", ph))
	}
	if params.Brand != nil && strings.TrimSpace(*params.Brand) != "" {
		ph := addArg(strings.TrimSpace(*params.Brand))
		clauses = append(clauses, fmt.Sprintf("b.slug = %s", ph))
	}
	if params.Featured != nil {
		ph := addArg(*params.Featured)
		clauses = append(clauses, fmt.Sprintf("p.is_featured = %s", ph))
	}

	whereSQL := strings.Join(clauses, " AND ")
	var total int
	_ = s.pool.QueryRow(r.Context(), fmt.Sprintf(`
		SELECT count(*)::int FROM products p JOIN brands b ON b.id = p.brand_id JOIN categories c ON c.id = p.category_id WHERE %s
	`, whereSQL), args...).Scan(&total)

	offset := (page - 1) * pageSize
	listArgs := append([]any{}, args...)
	listArgs = append(listArgs, pageSize, offset)

	rows, err := s.pool.Query(r.Context(), fmt.Sprintf(`
		SELECT
			p.id, p.slug, p.name, p.is_active, p.is_featured, (p.deleted_at IS NOT NULL) AS is_archived,
			p.version, p.updated_at,
			b.id, b.slug, b.name,
			c.id, c.slug, c.name,
			COALESCE((SELECT v.sku FROM variants v WHERE v.product_id = p.id ORDER BY v.created_at ASC LIMIT 1), '') AS primary_sku,
			(SELECT v.id FROM variants v WHERE v.product_id = p.id ORDER BY v.created_at ASC LIMIT 1) AS primary_vid,
			(SELECT pi.url FROM product_images pi WHERE pi.product_id = p.id ORDER BY pi.sort_order ASC LIMIT 1) AS img_url,
			COALESCE((SELECT min(v.price_idr) FROM variants v WHERE v.product_id = p.id), 0)::bigint AS min_price,
			COALESCE((SELECT max(v.price_idr) FROM variants v WHERE v.product_id = p.id), 0)::bigint AS max_price,
			COALESCE((SELECT sum(v.stock) FROM variants v WHERE v.product_id = p.id), 0)::int AS total_stock
		FROM products p
		JOIN brands b ON b.id = p.brand_id
		JOIN categories c ON c.id = p.category_id
		WHERE %s
		ORDER BY p.updated_at DESC
		LIMIT $%d OFFSET $%d
	`, whereSQL, len(listArgs)-1, len(listArgs)), listArgs...)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	defer rows.Close()

	items := make([]openapi.AdminProductRow, 0)
	for rows.Next() {
		var row openapi.AdminProductRow
		var ver, stock int32
		var pvid uuid.NullUUID
		if err := rows.Scan(
			&row.Id, &row.Slug, &row.Name, &row.IsActive, &row.IsFeatured, &row.IsArchived,
			&ver, &row.UpdatedAt,
			&row.Brand.Id, &row.Brand.Slug, &row.Brand.Name,
			&row.Category.Id, &row.Category.Slug, &row.Category.Name,
			&row.PrimarySku, &pvid, &row.ImageUrl,
			&row.PriceFromIdr, &row.PriceToIdr, &stock,
		); err == nil {
			row.Version = int(ver)
			row.TotalStock = int(stock)
			if pvid.Valid {
				u := pvid.UUID
				row.PrimaryVariantId = &u
			}
			row.StockStatus = inventory.ComputeStockStatus(stock, 3, false)
			items = append(items, row)
		}
	}

	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}
	platform.WritePrivateJSON(w, http.StatusOK, openapi.AdminProductListResponse{
		Items:      items,
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
	})
}

func (s *Server) buildAdminProductDetail(ctx context.Context, id uuid.UUID, canFinance bool) (*openapi.AdminProduct, error) {
	p, err := s.q.GetAdminProductByID(ctx, id)
	if err != nil {
		return nil, platform.NewAppError(http.StatusNotFound, openapi.NotFound, "Produk tidak ditemukan")
	}

	imgRows, _ := s.q.ListProductImages(ctx, p.ID)
	images := make([]openapi.Image, 0, len(imgRows))
	for _, im := range imgRows {
		var wPtr, hPtr *int
		if im.Width != nil {
			v := int(*im.Width)
			wPtr = &v
		}
		if im.Height != nil {
			v := int(*im.Height)
			hPtr = &v
		}
		images = append(images, openapi.Image{
			Url:    im.Url,
			Alt:    im.Alt,
			Width:  wPtr,
			Height: hPtr,
			Kind:   openapi.ImageKind(im.Kind),
		})
	}

	optRows, _ := s.q.ListProductOptions(ctx, p.ID)
	valRows, _ := s.q.ListProductOptionValuesByProduct(ctx, p.ID)
	valByOpt := make(map[uuid.UUID][]openapi.ProductOptionValue)
	for _, vr := range valRows {
		valByOpt[vr.OptionID] = append(valByOpt[vr.OptionID], openapi.ProductOptionValue{
			Id:        vr.ID,
			Label:     vr.Label,
			SwatchHex: vr.SwatchHex,
		})
	}
	options := make([]openapi.ProductOption, 0, len(optRows))
	for _, op := range optRows {
		vals := valByOpt[op.ID]
		if vals == nil {
			vals = []openapi.ProductOptionValue{}
		}
		options = append(options, openapi.ProductOption{
			Id:      op.ID,
			Name:    op.Name,
			Display: openapi.OptionDisplay(op.Display),
			Values:  vals,
		})
	}

	varRows, _ := s.q.ListAllVariantsByProduct(ctx, p.ID)
	variants := make([]openapi.AdminVariant, 0, len(varRows))
	for _, vr := range varRows {
		ovIDs := vr.OptionValueIds
		if ovIDs == nil {
			ovIDs = []uuid.UUID{}
		}
		var costPtr *int64
		if canFinance {
			costPtr = vr.CostIdr
		}
		variants = append(variants, openapi.AdminVariant{
			Id:                vr.ID,
			Sku:               vr.Sku,
			Barcode:           vr.Barcode,
			OptionValueIds:    ovIDs,
			PriceIdr:          vr.PriceIdr,
			CompareAtPriceIdr: vr.CompareAtPriceIdr,
			CostIdr:           costPtr,
			Stock:             int(vr.Stock),
			LowStockThreshold: int(vr.LowStockThreshold),
			IsPreorder:        vr.IsPreorder,
			IsActive:          vr.IsActive,
			ImageUrl:          vr.ImageUrl,
		})
	}

	specs := make(map[string]string)
	_ = json.Unmarshal(p.Specs, &specs)
	highlights := p.Highlights
	if highlights == nil {
		highlights = []string{}
	}

	return &openapi.AdminProduct{
		Id:                p.ID,
		Slug:              p.Slug,
		Name:              p.Name,
		Summary:           p.Summary,
		Description:       p.Description,
		Highlights:        highlights,
		Specs:             specs,
		BrandId:           p.BrandID,
		CategoryId:        p.CategoryID,
		IsFeatured:        p.IsFeatured,
		IsActive:          p.IsActive,
		SeoTitle:          p.SeoTitle,
		SeoDescription:    p.SeoDescription,
		Version:           int(p.Version),
		Images:            images,
		Options:           options,
		Variants:          variants,
		RelatedProductIds: []uuid.UUID{},
	}, nil
}

// AdminGetProduct implements GET /v1/admin/products/{id}
func (s *Server) AdminGetProduct(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	_, user, ok := s.authSvc.RequirePermission(w, r, rbac.PermProductsRead)
	if !ok {
		return
	}
	resp, err := s.buildAdminProductDetail(r.Context(), id, rbac.HasPermission(user.Role, rbac.PermFinanceView))
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	platform.WritePrivateJSON(w, http.StatusOK, resp)
}

// AdminCreateProduct implements POST /v1/admin/products
func (s *Server) AdminCreateProduct(w http.ResponseWriter, r *http.Request) {
	_, user, ok := s.authSvc.RequirePermission(w, r, rbac.PermProductsWrite)
	if !ok {
		return
	}
	var req openapi.AdminProductInput
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

	prodID := platform.NewUUID()
	specsBytes, _ := json.Marshal(req.Specs)
	if len(specsBytes) == 0 || string(specsBytes) == "null" {
		specsBytes = []byte("{}")
	}
	highlights := []string{}
	if req.Highlights != nil {
		highlights = *req.Highlights
	}

	_, err = tx.Exec(r.Context(), `
		INSERT INTO products (id, slug, name, brand_id, category_id, summary, description, highlights, specs, is_featured, is_active, seo_title, seo_description, version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, 1)
	`, prodID, strings.TrimSpace(req.Slug), strings.TrimSpace(req.Name), req.BrandId, req.CategoryId, req.Summary, req.Description, highlights, specsBytes, req.IsFeatured, req.IsActive, req.SeoTitle, req.SeoDescription)
	if err != nil {
		platform.WriteError(w, r, platform.NewAppError(http.StatusConflict, openapi.Conflict, "Slug produk sudah digunakan"))
		return
	}

	if req.Images != nil {
		for i, im := range *req.Images {
			_, _ = tx.Exec(r.Context(), `
				INSERT INTO product_images (id, product_id, url, alt, width, height, sort_order, kind)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			`, platform.NewUUID(), prodID, im.Url, im.Alt, im.Width, im.Height, i, string(im.Kind))
		}
	}

	if req.Options != nil {
		for i, op := range *req.Options {
			optID := op.Id
			if optID == uuid.Nil {
				optID = platform.NewUUID()
			}
			_, _ = tx.Exec(r.Context(), `
				INSERT INTO product_options (id, product_id, name, sort_order, display)
				VALUES ($1, $2, $3, $4, $5)
			`, optID, prodID, op.Name, i, string(op.Display))
			for j, val := range op.Values {
				valID := val.Id
				if valID == uuid.Nil {
					valID = platform.NewUUID()
				}
				_, _ = tx.Exec(r.Context(), `
					INSERT INTO product_option_values (id, option_id, label, swatch_hex, sort_order)
					VALUES ($1, $2, $3, $4, $5)
				`, valID, optID, val.Label, val.SwatchHex, j)
			}
		}
	}

	for _, vr := range req.Variants {
		varID := platform.NewUUID()
		if vr.Id != nil && *vr.Id != uuid.Nil {
			varID = *vr.Id
		}
		var ovIDs []uuid.UUID
		if vr.OptionValueIds != nil {
			for _, sID := range *vr.OptionValueIds {
				if parsed, err := uuid.Parse(sID); err == nil {
					ovIDs = append(ovIDs, parsed)
				}
			}
		}
		if ovIDs == nil {
			ovIDs = []uuid.UUID{}
		}
		thresh := 3
		if vr.LowStockThreshold != nil {
			thresh = *vr.LowStockThreshold
		}
		isPre := vr.IsPreorder != nil && *vr.IsPreorder
		isAct := vr.IsActive == nil || *vr.IsActive

		_, err := tx.Exec(r.Context(), `
			INSERT INTO variants (id, product_id, sku, barcode, option_value_ids, price_idr, compare_at_price_idr, cost_idr, stock, low_stock_threshold, is_preorder, is_active)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 0, $9, $10, $11)
		`, varID, prodID, strings.TrimSpace(vr.Sku), vr.Barcode, ovIDs, vr.PriceIdr, vr.CompareAtPriceIdr, vr.CostIdr, thresh, isPre, isAct)
		if err != nil {
			platform.WriteError(w, r, platform.NewAppError(http.StatusConflict, openapi.Conflict, fmt.Sprintf("SKU %s sudah digunakan", vr.Sku)))
			return
		}
		if vr.Stock > 0 {
			if _, err := s.invSvc.Adjust(r.Context(), qtx, inventory.AdjustParams{
				VariantID: varID,
				Delta:     int32(vr.Stock),
				Reason:    "initial",
				ActorID:   uuid.NullUUID{UUID: user.ID, Valid: true},
				Note:      "Stok awal pembuatan produk",
			}); err != nil {
				platform.WriteError(w, r, err)
				return
			}
		}
	}

	_ = audit.Record(r.Context(), qtx, audit.Entry{
		ActorID:    uuid.NullUUID{UUID: user.ID, Valid: true},
		ActorEmail: user.Email,
		ActorRole:  user.Role,
		Action:     "product.create",
		EntityType: "product",
		EntityID:   prodID.String(),
		Summary:    fmt.Sprintf("Membuat produk baru: %s (%s)", req.Name, req.Slug),
		After:      map[string]any{"name": req.Name, "slug": req.Slug},
	})

	if err := tx.Commit(r.Context()); err != nil {
		platform.WriteError(w, r, err)
		return
	}

	s.InvalidateCatalogCache()
	resp, err := s.buildAdminProductDetail(r.Context(), prodID, rbac.HasPermission(user.Role, rbac.PermFinanceView))
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	platform.WritePrivateJSON(w, http.StatusCreated, resp)
}

// AdminUpdateProduct implements PATCH /v1/admin/products/{id}
func (s *Server) AdminUpdateProduct(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	_, user, ok := s.authSvc.RequirePermission(w, r, rbac.PermProductsWrite)
	if !ok {
		return
	}
	var req openapi.AdminProductInput
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

	existing, err := qtx.GetAdminProductByID(r.Context(), id)
	if err != nil {
		platform.WriteError(w, r, platform.NewAppError(http.StatusNotFound, openapi.NotFound, "Produk tidak ditemukan"))
		return
	}
	if req.Version != nil && *req.Version != int(existing.Version) {
		platform.WriteError(w, r, platform.NewAppError(http.StatusConflict, openapi.Conflict, "Data sudah diubah oleh orang lain. Muat ulang halaman."))
		return
	}

	specsBytes, _ := json.Marshal(req.Specs)
	if len(specsBytes) == 0 || string(specsBytes) == "null" {
		specsBytes = []byte("{}")
	}
	highlights := []string{}
	if req.Highlights != nil {
		highlights = *req.Highlights
	}

	_, err = tx.Exec(r.Context(), `
		UPDATE products
		SET slug = $2, name = $3, brand_id = $4, category_id = $5, summary = $6, description = $7,
		    highlights = $8, specs = $9, is_featured = $10, is_active = $11, seo_title = $12, seo_description = $13,
		    version = version + 1, updated_at = now()
		WHERE id = $1
	`, id, strings.TrimSpace(req.Slug), strings.TrimSpace(req.Name), req.BrandId, req.CategoryId, req.Summary, req.Description, highlights, specsBytes, req.IsFeatured, req.IsActive, req.SeoTitle, req.SeoDescription)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}

	if req.Images != nil {
		_, _ = tx.Exec(r.Context(), `DELETE FROM product_images WHERE product_id = $1`, id)
		for i, im := range *req.Images {
			_, _ = tx.Exec(r.Context(), `
				INSERT INTO product_images (id, product_id, url, alt, width, height, sort_order, kind)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			`, platform.NewUUID(), id, im.Url, im.Alt, im.Width, im.Height, i, string(im.Kind))
		}
	}

	for _, vr := range req.Variants {
		thresh := 3
		if vr.LowStockThreshold != nil {
			thresh = *vr.LowStockThreshold
		}
		isPre := vr.IsPreorder != nil && *vr.IsPreorder
		isAct := vr.IsActive == nil || *vr.IsActive

		if vr.Id != nil && *vr.Id != uuid.Nil {
			oldVar, err := qtx.LockVariantForUpdate(r.Context(), *vr.Id)
			if err == nil {
				_, _ = tx.Exec(r.Context(), `
					UPDATE variants
					SET sku = $2, barcode = $3, price_idr = $4, compare_at_price_idr = $5, cost_idr = COALESCE($6, cost_idr),
					    low_stock_threshold = $7, is_preorder = $8, is_active = $9
					WHERE id = $1
				`, *vr.Id, strings.TrimSpace(vr.Sku), vr.Barcode, vr.PriceIdr, vr.CompareAtPriceIdr, vr.CostIdr, thresh, isPre, isAct)

				delta := int32(vr.Stock) - oldVar.Stock
				if delta != 0 {
					_, _ = s.invSvc.Adjust(r.Context(), qtx, inventory.AdjustParams{
						VariantID: *vr.Id,
						Delta:     delta,
						Reason:    "correction",
						ActorID:   uuid.NullUUID{UUID: user.ID, Valid: true},
						Note:      "Perubahan stok dari editor produk",
					})
				}
			}
		} else {
			newVID := platform.NewUUID()
			_, _ = tx.Exec(r.Context(), `
				INSERT INTO variants (id, product_id, sku, barcode, price_idr, compare_at_price_idr, cost_idr, stock, low_stock_threshold, is_preorder, is_active)
				VALUES ($1, $2, $3, $4, $5, $6, $7, 0, $8, $9, $10)
			`, newVID, id, strings.TrimSpace(vr.Sku), vr.Barcode, vr.PriceIdr, vr.CompareAtPriceIdr, vr.CostIdr, thresh, isPre, isAct)
			if vr.Stock > 0 {
				_, _ = s.invSvc.Adjust(r.Context(), qtx, inventory.AdjustParams{
					VariantID: newVID,
					Delta:     int32(vr.Stock),
					Reason:    "initial",
					ActorID:   uuid.NullUUID{UUID: user.ID, Valid: true},
					Note:      "Varian baru ditambahkan",
				})
			}
		}
	}

	_ = audit.Record(r.Context(), qtx, audit.Entry{
		ActorID:    uuid.NullUUID{UUID: user.ID, Valid: true},
		ActorEmail: user.Email,
		ActorRole:  user.Role,
		Action:     "product.update",
		EntityType: "product",
		EntityID:   id.String(),
		Summary:    fmt.Sprintf("Memperbarui produk: %s", req.Name),
		Before:     map[string]any{"name": existing.Name, "slug": existing.Slug},
		After:      map[string]any{"name": req.Name, "slug": req.Slug},
	})

	if err := tx.Commit(r.Context()); err != nil {
		platform.WriteError(w, r, err)
		return
	}

	s.InvalidateCatalogCache()
	resp, err := s.buildAdminProductDetail(r.Context(), id, rbac.HasPermission(user.Role, rbac.PermFinanceView))
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	platform.WritePrivateJSON(w, http.StatusOK, resp)
}

// AdminArchiveProduct implements DELETE /v1/admin/products/{id}
func (s *Server) AdminArchiveProduct(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	_, user, ok := s.authSvc.RequirePermission(w, r, rbac.PermProductsDelete)
	if !ok {
		return
	}
	_, _ = s.pool.Exec(r.Context(), `UPDATE products SET deleted_at = now(), is_active = false, updated_at = now() WHERE id = $1`, id)
	_ = audit.Record(r.Context(), s.q, audit.Entry{
		ActorID:    uuid.NullUUID{UUID: user.ID, Valid: true},
		ActorEmail: user.Email,
		ActorRole:  user.Role,
		Action:     "product.archive",
		EntityType: "product",
		EntityID:   id.String(),
		Summary:    fmt.Sprintf("Mengarsipkan produk %s", id.String()),
	})
	s.InvalidateCatalogCache()
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusNoContent)
}

// AdminDuplicateProduct implements POST /v1/admin/products/{id}/duplicate
func (s *Server) AdminDuplicateProduct(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	_, user, ok := s.authSvc.RequirePermission(w, r, rbac.PermProductsWrite)
	if !ok {
		return
	}
	orig, err := s.buildAdminProductDetail(r.Context(), id, true)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	suffix := fmt.Sprintf("-copy-%d", time.Now().Unix()%10000)
	newID := platform.NewUUID()
	specsBytes, _ := json.Marshal(orig.Specs)
	_, err = s.pool.Exec(r.Context(), `
		INSERT INTO products (id, slug, name, brand_id, category_id, summary, description, highlights, specs, is_featured, is_active, version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, false, false, 1)
	`, newID, orig.Slug+suffix, orig.Name+" (Salinan)", orig.BrandId, orig.CategoryId, orig.Summary, orig.Description, orig.Highlights, specsBytes)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	for i, vr := range orig.Variants {
		_, _ = s.pool.Exec(r.Context(), `
			INSERT INTO variants (id, product_id, sku, price_idr, compare_at_price_idr, cost_idr, stock, low_stock_threshold, is_preorder, is_active)
			VALUES ($1, $2, $3, $4, $5, $6, 0, $7, $8, true)
		`, platform.NewUUID(), newID, fmt.Sprintf("%s-COPY-%d", vr.Sku, i+1), vr.PriceIdr, vr.CompareAtPriceIdr, vr.CostIdr, vr.LowStockThreshold, vr.IsPreorder)
	}
	_ = audit.Record(r.Context(), s.q, audit.Entry{
		ActorID:    uuid.NullUUID{UUID: user.ID, Valid: true},
		ActorEmail: user.Email,
		ActorRole:  user.Role,
		Action:     "product.duplicate",
		EntityType: "product",
		EntityID:   newID.String(),
		Summary:    fmt.Sprintf("Menduplikat produk %s menjadi %s", orig.Name, orig.Name+" (Salinan)"),
	})
	resp, _ := s.buildAdminProductDetail(r.Context(), newID, rbac.HasPermission(user.Role, rbac.PermFinanceView))
	platform.WritePrivateJSON(w, http.StatusCreated, resp)
}

// AdminBulkProducts implements POST /v1/admin/products/bulk
func (s *Server) AdminBulkProducts(w http.ResponseWriter, r *http.Request) {
	_, user, ok := s.authSvc.RequirePermission(w, r, rbac.PermProductsWrite)
	if !ok {
		return
	}
	var req openapi.AdminBulkProductsRequest
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	ctx := r.Context()
	affected := 0

	switch req.Action {
	case openapi.Activate:
		tag, _ := s.pool.Exec(ctx, `UPDATE products SET is_active = true, updated_at = now() WHERE id = ANY($1)`, req.ProductIds)
		affected = int(tag.RowsAffected())
	case openapi.Deactivate:
		tag, _ := s.pool.Exec(ctx, `UPDATE products SET is_active = false, updated_at = now() WHERE id = ANY($1)`, req.ProductIds)
		affected = int(tag.RowsAffected())
	case openapi.Archive:
		tag, _ := s.pool.Exec(ctx, `UPDATE products SET deleted_at = now(), is_active = false, updated_at = now() WHERE id = ANY($1)`, req.ProductIds)
		affected = int(tag.RowsAffected())
	case openapi.Restore:
		tag, _ := s.pool.Exec(ctx, `UPDATE products SET deleted_at = NULL, updated_at = now() WHERE id = ANY($1)`, req.ProductIds)
		affected = int(tag.RowsAffected())
	case openapi.MoveCategory:
		if req.CategoryId != nil {
			tag, _ := s.pool.Exec(ctx, `UPDATE products SET category_id = $2, updated_at = now() WHERE id = ANY($1)`, req.ProductIds, *req.CategoryId)
			affected = int(tag.RowsAffected())
		}
	case openapi.SetBrand:
		if req.BrandId != nil {
			tag, _ := s.pool.Exec(ctx, `UPDATE products SET brand_id = $2, updated_at = now() WHERE id = ANY($1)`, req.ProductIds, *req.BrandId)
			affected = int(tag.RowsAffected())
		}
	case openapi.PriceAdjust:
		kind := "percent"
		if req.PriceAdjustKind != nil {
			kind = string(*req.PriceAdjustKind)
		}
		var val int64
		if req.PriceAdjustValue != nil {
			val = *req.PriceAdjustValue
		}
		var roundUnit int64 = 1000
		if req.RoundingUnit != nil {
			roundUnit = int64(*req.RoundingUnit)
		}
		rows, err := s.pool.Query(ctx, `SELECT id, price_idr FROM variants WHERE product_id = ANY($1)`, req.ProductIds)
		if err == nil {
			type vPrice struct {
				id    uuid.UUID
				price int64
			}
			var vps []vPrice
			for rows.Next() {
				var vp vPrice
				if rows.Scan(&vp.id, &vp.price) == nil {
					vps = append(vps, vp)
				}
			}
			rows.Close()
			for _, vp := range vps {
				nextP := orders.AdjustAndRoundPrice(vp.price, kind, val, roundUnit)
				_, _ = s.pool.Exec(ctx, `UPDATE variants SET price_idr = $2 WHERE id = $1`, vp.id, nextP)
				affected++
			}
		}
	}

	_ = audit.Record(ctx, s.q, audit.Entry{
		ActorID:    uuid.NullUUID{UUID: user.ID, Valid: true},
		ActorEmail: user.Email,
		ActorRole:  user.Role,
		Action:     "product.bulk_" + string(req.Action),
		EntityType: "product",
		Summary:    fmt.Sprintf("Aksi massal %s pada %d produk", req.Action, len(req.ProductIds)),
	})
	s.InvalidateCatalogCache()
	platform.WritePrivateJSON(w, http.StatusOK, map[string]int{"affectedCount": affected})
}

// AdminImportProductsCsv implements POST /v1/admin/products/import
func (s *Server) AdminImportProductsCsv(w http.ResponseWriter, r *http.Request, params openapi.AdminImportProductsCsvParams) {
	_, user, ok := s.authSvc.RequirePermission(w, r, rbac.PermProductsWrite)
	if !ok {
		return
	}
	var body struct {
		CsvContent string `json:"csvContent"`
	}
	if err := platform.DecodeStrictJSON(r, &body); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	dryRun := params.DryRun == nil || *params.DryRun

	sum := sha256.Sum256([]byte(body.CsvContent))
	csvHash := hex.EncodeToString(sum[:])

	reader := csv.NewReader(strings.NewReader(body.CsvContent))
	records, err := reader.ReadAll()
	if err != nil || len(records) < 2 {
		platform.WriteError(w, r, platform.NewValidationError("Format CSV tidak valid atau kosong", nil))
		return
	}

	headers := make(map[string]int)
	for i, h := range records[0] {
		headers[strings.ToLower(strings.TrimSpace(h))] = i
	}
	getCol := func(row []string, name string) string {
		if idx, ok := headers[name]; ok && idx < len(row) {
			return strings.TrimSpace(row[idx])
		}
		return ""
	}

	resp := openapi.AdminProductImportResponse{
		Errors: []struct {
			Field   string `json:"field"`
			Message string `json:"message"`
			Row     int    `json:"row"`
		}{},
	}

	for rowIdx := 1; rowIdx < len(records); rowIdx++ {
		rec := records[rowIdx]
		sku := getCol(rec, "sku")
		name := getCol(rec, "name")
		priceStr := getCol(rec, "price_idr")

		if sku == "" {
			resp.Errors = append(resp.Errors, struct {
				Field   string `json:"field"`
				Message string `json:"message"`
				Row     int    `json:"row"`
			}{Row: rowIdx + 1, Field: "sku", Message: "SKU wajib diisi"})
			continue
		}
		if name == "" {
			resp.Errors = append(resp.Errors, struct {
				Field   string `json:"field"`
				Message string `json:"message"`
				Row     int    `json:"row"`
			}{Row: rowIdx + 1, Field: "name", Message: "Nama produk wajib diisi"})
			continue
		}
		price, err := strconv.ParseInt(priceStr, 10, 64)
		if err != nil || price < 0 {
			resp.Errors = append(resp.Errors, struct {
				Field   string `json:"field"`
				Message string `json:"message"`
				Row     int    `json:"row"`
			}{Row: rowIdx + 1, Field: "price_idr", Message: "Harga harus berupa angka bulat >= 0"})
			continue
		}

		if _, err := s.q.GetVariantBySKU(r.Context(), sku); err == nil {
			resp.ToUpdate++
		} else {
			resp.ToCreate++
		}
	}

	if dryRun {
		tok := platform.NewUUID().String()
		s.mu.Lock()
		s.importTokens[tok] = importDryRunToken{
			csvHash:   csvHash,
			expiresAt: time.Now().Add(15 * time.Minute),
		}
		s.mu.Unlock()
		resp.Token = &tok
		platform.WritePrivateJSON(w, http.StatusOK, resp)
		return
	}

	// Commit mode: verify dry-run token
	if params.Token == nil || *params.Token == "" {
		platform.WriteError(w, r, platform.NewAppError(http.StatusBadRequest, openapi.BadRequest, "Token dry-run wajib disertakan untuk konfirmasi impor"))
		return
	}
	s.mu.Lock()
	storedTok, tokOK := s.importTokens[*params.Token]
	delete(s.importTokens, *params.Token)
	s.mu.Unlock()
	if !tokOK || storedTok.csvHash != csvHash || time.Now().After(storedTok.expiresAt) {
		platform.WriteError(w, r, platform.NewAppError(http.StatusConflict, openapi.Conflict, "Token impor sudah kedaluwarsa atau isi CSV berubah, silakan ulangi dry-run"))
		return
	}
	if len(resp.Errors) > 0 {
		platform.WriteError(w, r, platform.NewValidationError("Terdapat baris CSV yang tidak valid", nil))
		return
	}

	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	qtx := s.q.WithTx(tx)

	for rowIdx := 1; rowIdx < len(records); rowIdx++ {
		rec := records[rowIdx]
		sku := getCol(rec, "sku")
		price, _ := strconv.ParseInt(getCol(rec, "price_idr"), 10, 64)
		stock, _ := strconv.Atoi(getCol(rec, "stock"))

		if vr, err := qtx.GetVariantBySKU(r.Context(), sku); err == nil {
			_, _ = tx.Exec(r.Context(), `UPDATE variants SET price_idr = $2 WHERE id = $1`, vr.ID, price)
			delta := int32(stock) - vr.Stock
			if delta != 0 {
				_, _ = s.invSvc.Adjust(r.Context(), qtx, inventory.AdjustParams{
					VariantID: vr.ID,
					Delta:     delta,
					Reason:    "import",
					ActorID:   uuid.NullUUID{UUID: user.ID, Valid: true},
					Note:      "Impor CSV katalog",
				})
			}
		}
	}

	_ = audit.Record(r.Context(), qtx, audit.Entry{
		ActorID:    uuid.NullUUID{UUID: user.ID, Valid: true},
		ActorEmail: user.Email,
		ActorRole:  user.Role,
		Action:     "product.import_csv",
		EntityType: "product",
		Summary:    fmt.Sprintf("Impor CSV katalog selesai (%d dibuat, %d diperbarui)", resp.ToCreate, resp.ToUpdate),
	})

	if err := tx.Commit(r.Context()); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	s.InvalidateCatalogCache()
	platform.WritePrivateJSON(w, http.StatusOK, resp)
}

// AdminExportProductsCsv implements GET /v1/admin/products/export.csv
func (s *Server) AdminExportProductsCsv(w http.ResponseWriter, r *http.Request, params openapi.AdminExportProductsCsvParams) {
	_, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermProductsRead)
	if !ok {
		return
	}
	rows, err := s.pool.Query(r.Context(), `
		SELECT v.sku, p.name, b.name, c.slug, v.price_idr, COALESCE(v.compare_at_price_idr, 0), v.stock, p.is_featured
		FROM variants v
		JOIN products p ON p.id = v.product_id
		JOIN brands b ON b.id = p.brand_id
		JOIN categories c ON c.id = p.category_id
		WHERE p.deleted_at IS NULL
		ORDER BY p.name ASC
	`)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	defer rows.Close()

	var csvRows [][]string
	for rows.Next() {
		var sku, name, brand, cat string
		var price, cmp int64
		var stock int
		var feat bool
		if rows.Scan(&sku, &name, &brand, &cat, &price, &cmp, &stock, &feat) == nil {
			cmpStr := ""
			if cmp > 0 {
				cmpStr = strconv.FormatInt(cmp, 10)
			}
			csvRows = append(csvRows, []string{
				sku, name, brand, cat, strconv.FormatInt(price, 10), cmpStr, strconv.Itoa(stock), strconv.FormatBool(feat),
			})
		}
	}
	var delim *string
	if params.Delimiter != nil {
		d := string(*params.Delimiter)
		delim = &d
	}
	writeCSVDownload(w, r, "gmc-products.csv", delim, []string{
		"sku", "name", "brand", "category", "price_idr", "compare_at_price_idr", "stock", "is_featured",
	}, csvRows)
}

// AdminGetProductImportTemplateCsv implements GET /v1/admin/products/import/template.csv
func (s *Server) AdminGetProductImportTemplateCsv(w http.ResponseWriter, r *http.Request) {
	_, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermProductsRead)
	if !ok {
		return
	}
	writeCSVDownload(w, r, "gmc-import-template.csv", nil, []string{
		"sku", "name", "brand", "category", "price_idr", "compare_at_price_idr", "stock", "cost_idr", "low_stock_threshold", "barcode", "is_featured",
	}, [][]string{
		{"COR-DDR5-32G-6000", "Corsair Vengeance DDR5 32GB (2x16GB) 6000MHz", "Corsair", "ram", "1899000", "2099000", "12", "1650000", "3", "840006659812", "true"},
	})
}

// AdminUploadImage implements POST /v1/admin/uploads
func (s *Server) AdminUploadImage(w http.ResponseWriter, r *http.Request) {
	_, user, ok := s.authSvc.RequirePermission(w, r, rbac.PermProductsWrite)
	if !ok {
		return
	}
	if err := r.ParseMultipartForm(6 << 20); err != nil {
		platform.WriteError(w, r, platform.NewAppError(http.StatusBadRequest, openapi.BadRequest, "Gagal membaca form multipart (maks 5 MB)"))
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		platform.WriteError(w, r, platform.NewAppError(http.StatusBadRequest, openapi.BadRequest, "Field file wajib disertakan"))
		return
	}
	defer file.Close()

	newID := platform.NewUUID()
	proc, err := s.storage.ProcessAndSave(newID, file)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}

	// Deduplicate by SHA-256 if identical file already exists
	if existing, err := s.q.GetUploadBySHA256(r.Context(), proc.SHA256); err == nil {
		_ = s.storage.Delete(newID)
		_ = s.storage.SaveExisting(existing.ID, proc.RawJPEG)
		platform.WritePrivateJSON(w, http.StatusCreated, openapi.UploadResponse{
			Id:     existing.ID,
			Url:    platform.UploadURL(existing.ID, 800),
			Width:  int(existing.Width),
			Height: int(existing.Height),
		})
		return
	}

	up, err := s.q.CreateUpload(r.Context(), db.CreateUploadParams{
		ID:           newID,
		OriginalName: header.Filename,
		Mime:         proc.Mime,
		Bytes:        int32(proc.Bytes),
		Width:        int32(proc.Width),
		Height:       int32(proc.Height),
		Sha256:       proc.SHA256,
		CreatedBy:    uuid.NullUUID{UUID: user.ID, Valid: true},
	})
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}

	platform.WritePrivateJSON(w, http.StatusCreated, openapi.UploadResponse{
		Id:     up.ID,
		Url:    platform.UploadURL(up.ID, 800),
		Width:  int(up.Width),
		Height: int(up.Height),
	})
}

// AdminDeleteUpload implements DELETE /v1/admin/uploads/{id}
func (s *Server) AdminDeleteUpload(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	_, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermProductsWrite)
	if !ok {
		return
	}
	_ = s.q.DeleteUpload(r.Context(), id)
	_ = s.storage.Delete(id)
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusNoContent)
}

// AdminListCategories implements GET /v1/admin/categories
func (s *Server) AdminListCategories(w http.ResponseWriter, r *http.Request) {
	_, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermProductsRead)
	if !ok {
		return
	}
	rows, err := s.q.ListAdminCategories(r.Context())
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	items := make([]openapi.AdminCategory, 0, len(rows))
	for _, row := range rows {
		var pid *uuid.UUID
		if row.ParentID.Valid {
			u := row.ParentID.UUID
			pid = &u
		}
		items = append(items, openapi.AdminCategory{
			Id:           row.ID,
			Slug:         row.Slug,
			Name:         row.Name,
			ParentId:     pid,
			Description:  row.Description,
			ImageUrl:     row.ImageUrl,
			SortOrder:    int(row.SortOrder),
			IsActive:     row.IsActive,
			ProductCount: int(row.ProductCount),
		})
	}
	platform.WritePrivateJSON(w, http.StatusOK, map[string]any{"items": items})
}

// AdminCreateCategory implements POST /v1/admin/categories
func (s *Server) AdminCreateCategory(w http.ResponseWriter, r *http.Request) {
	_, user, ok := s.authSvc.RequirePermission(w, r, rbac.PermProductsWrite)
	if !ok {
		return
	}
	var req openapi.AdminCategoryInput
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	var parentID uuid.NullUUID
	if req.ParentId != nil {
		parentID = uuid.NullUUID{UUID: *req.ParentId, Valid: true}
	}
	so := 0
	if req.SortOrder != nil {
		so = *req.SortOrder
	}
	row, err := s.q.CreateCategory(r.Context(), db.CreateCategoryParams{
		ID:          platform.NewUUID(),
		Slug:        strings.TrimSpace(req.Slug),
		Name:        strings.TrimSpace(req.Name),
		ParentID:    parentID,
		SortOrder:   int32(so),
		Description: req.Description,
		ImageUrl:    req.ImageUrl,
		IsActive:    req.IsActive,
	})
	if err != nil {
		platform.WriteError(w, r, platform.NewAppError(http.StatusConflict, openapi.Conflict, "Slug kategori sudah digunakan"))
		return
	}
	_ = audit.Record(r.Context(), s.q, audit.Entry{
		ActorID:    uuid.NullUUID{UUID: user.ID, Valid: true},
		ActorEmail: user.Email,
		ActorRole:  user.Role,
		Action:     "category.create",
		EntityType: "category",
		EntityID:   row.ID.String(),
		Summary:    fmt.Sprintf("Membuat kategori %s", row.Name),
	})
	s.InvalidateCatalogCache()
	platform.WritePrivateJSON(w, http.StatusCreated, openapi.AdminCategory{
		Id:           row.ID,
		Slug:         row.Slug,
		Name:         row.Name,
		Description:  row.Description,
		ImageUrl:     row.ImageUrl,
		SortOrder:    int(row.SortOrder),
		IsActive:     row.IsActive,
		ProductCount: 0,
	})
}

// AdminUpdateCategory implements PATCH /v1/admin/categories/{id}
func (s *Server) AdminUpdateCategory(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	_, user, ok := s.authSvc.RequirePermission(w, r, rbac.PermProductsWrite)
	if !ok {
		return
	}
	var req openapi.AdminCategoryInput
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	var parentID uuid.NullUUID
	if req.ParentId != nil {
		parentID = uuid.NullUUID{UUID: *req.ParentId, Valid: true}
	}
	so := 0
	if req.SortOrder != nil {
		so = *req.SortOrder
	}
	row, err := s.q.UpdateCategory(r.Context(), db.UpdateCategoryParams{
		ID:          id,
		Slug:        strings.TrimSpace(req.Slug),
		Name:        strings.TrimSpace(req.Name),
		ParentID:    parentID,
		SortOrder:   int32(so),
		Description: req.Description,
		ImageUrl:    req.ImageUrl,
		IsActive:    req.IsActive,
	})
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	_ = audit.Record(r.Context(), s.q, audit.Entry{
		ActorID:    uuid.NullUUID{UUID: user.ID, Valid: true},
		ActorEmail: user.Email,
		ActorRole:  user.Role,
		Action:     "category.update",
		EntityType: "category",
		EntityID:   row.ID.String(),
		Summary:    fmt.Sprintf("Memperbarui kategori %s", row.Name),
	})
	s.InvalidateCatalogCache()
	platform.WritePrivateJSON(w, http.StatusOK, openapi.AdminCategory{
		Id:          row.ID,
		Slug:        row.Slug,
		Name:        row.Name,
		Description: row.Description,
		ImageUrl:    row.ImageUrl,
		SortOrder:   int(row.SortOrder),
		IsActive:    row.IsActive,
	})
}

// AdminReorderCategories implements POST /v1/admin/categories/reorder
func (s *Server) AdminReorderCategories(w http.ResponseWriter, r *http.Request) {
	_, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermProductsWrite)
	if !ok {
		return
	}
	var req openapi.ReorderRequest
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	for i, id := range req.Ids {
		_ = s.q.UpdateCategorySortOrder(r.Context(), db.UpdateCategorySortOrderParams{
			ID:        id,
			SortOrder: int32(i),
		})
	}
	s.InvalidateCatalogCache()
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusNoContent)
}

// AdminDeleteCategory implements DELETE /v1/admin/categories/{id}
func (s *Server) AdminDeleteCategory(w http.ResponseWriter, r *http.Request, id openapi_types.UUID, params openapi.AdminDeleteCategoryParams) {
	_, user, ok := s.authSvc.RequirePermission(w, r, rbac.PermProductsWrite)
	if !ok {
		return
	}
	if params.MoveToCategoryId != nil {
		_ = s.q.MoveCategoryProducts(r.Context(), db.MoveCategoryProductsParams{
			CategoryID:   id,
			CategoryID_2: *params.MoveToCategoryId,
		})
	}
	if err := s.q.DeleteCategory(r.Context(), id); err != nil {
		platform.WriteError(w, r, platform.NewAppError(http.StatusConflict, openapi.Conflict, "Kategori masih memiliki produk aktif. Pindahkan produk terlebih dahulu."))
		return
	}
	_ = audit.Record(r.Context(), s.q, audit.Entry{
		ActorID:    uuid.NullUUID{UUID: user.ID, Valid: true},
		ActorEmail: user.Email,
		ActorRole:  user.Role,
		Action:     "category.delete",
		EntityType: "category",
		EntityID:   id.String(),
		Summary:    "Menghapus kategori",
	})
	s.InvalidateCatalogCache()
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusNoContent)
}

// AdminListBrands implements GET /v1/admin/brands
func (s *Server) AdminListBrands(w http.ResponseWriter, r *http.Request) {
	_, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermProductsRead)
	if !ok {
		return
	}
	rows, err := s.q.ListAdminBrands(r.Context())
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	items := make([]openapi.AdminBrand, 0, len(rows))
	for _, row := range rows {
		items = append(items, openapi.AdminBrand{
			Id:           row.ID,
			Slug:         row.Slug,
			Name:         row.Name,
			LogoText:     row.LogoText,
			IsActive:     row.IsActive,
			ProductCount: int(row.ProductCount),
		})
	}
	platform.WritePrivateJSON(w, http.StatusOK, map[string]any{"items": items})
}

// AdminCreateBrand implements POST /v1/admin/brands
func (s *Server) AdminCreateBrand(w http.ResponseWriter, r *http.Request) {
	_, user, ok := s.authSvc.RequirePermission(w, r, rbac.PermProductsWrite)
	if !ok {
		return
	}
	var req openapi.AdminBrandInput
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	row, err := s.q.CreateBrand(r.Context(), db.CreateBrandParams{
		ID:       platform.NewUUID(),
		Slug:     strings.TrimSpace(req.Slug),
		Name:     strings.TrimSpace(req.Name),
		LogoText: req.LogoText,
		IsActive: req.IsActive,
	})
	if err != nil {
		platform.WriteError(w, r, platform.NewAppError(http.StatusConflict, openapi.Conflict, "Slug brand sudah digunakan"))
		return
	}
	_ = audit.Record(r.Context(), s.q, audit.Entry{
		ActorID:    uuid.NullUUID{UUID: user.ID, Valid: true},
		ActorEmail: user.Email,
		ActorRole:  user.Role,
		Action:     "brand.create",
		EntityType: "brand",
		EntityID:   row.ID.String(),
		Summary:    fmt.Sprintf("Membuat brand %s", row.Name),
	})
	s.InvalidateCatalogCache()
	platform.WritePrivateJSON(w, http.StatusCreated, openapi.AdminBrand{
		Id:           row.ID,
		Slug:         row.Slug,
		Name:         row.Name,
		LogoText:     row.LogoText,
		IsActive:     row.IsActive,
		ProductCount: 0,
	})
}

// AdminUpdateBrand implements PATCH /v1/admin/brands/{id}
func (s *Server) AdminUpdateBrand(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	_, user, ok := s.authSvc.RequirePermission(w, r, rbac.PermProductsWrite)
	if !ok {
		return
	}
	var req openapi.AdminBrandInput
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	row, err := s.q.UpdateBrand(r.Context(), db.UpdateBrandParams{
		ID:       id,
		Slug:     strings.TrimSpace(req.Slug),
		Name:     strings.TrimSpace(req.Name),
		LogoText: req.LogoText,
		IsActive: req.IsActive,
	})
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	_ = audit.Record(r.Context(), s.q, audit.Entry{
		ActorID:    uuid.NullUUID{UUID: user.ID, Valid: true},
		ActorEmail: user.Email,
		ActorRole:  user.Role,
		Action:     "brand.update",
		EntityType: "brand",
		EntityID:   row.ID.String(),
		Summary:    fmt.Sprintf("Memperbarui brand %s", row.Name),
	})
	s.InvalidateCatalogCache()
	platform.WritePrivateJSON(w, http.StatusOK, openapi.AdminBrand{
		Id:       row.ID,
		Slug:     row.Slug,
		Name:     row.Name,
		LogoText: row.LogoText,
		IsActive: row.IsActive,
	})
}

// AdminDeleteBrand implements DELETE /v1/admin/brands/{id}
func (s *Server) AdminDeleteBrand(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	_, user, ok := s.authSvc.RequirePermission(w, r, rbac.PermProductsWrite)
	if !ok {
		return
	}
	if err := s.q.DeleteBrand(r.Context(), id); err != nil {
		platform.WriteError(w, r, platform.NewAppError(http.StatusConflict, openapi.Conflict, "Brand masih digunakan oleh produk"))
		return
	}
	_ = audit.Record(r.Context(), s.q, audit.Entry{
		ActorID:    uuid.NullUUID{UUID: user.ID, Valid: true},
		ActorEmail: user.Email,
		ActorRole:  user.Role,
		Action:     "brand.delete",
		EntityType: "brand",
		EntityID:   id.String(),
		Summary:    "Menghapus brand",
	})
	s.InvalidateCatalogCache()
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusNoContent)
}

// Suppress unused import warning
var _ = sort.Strings
var _ = pgx.ErrNoRows
