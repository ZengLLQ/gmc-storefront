package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"gmc/api/internal/admin/rbac"
	"gmc/api/internal/audit"
	"gmc/api/internal/gen/db"
	"gmc/api/internal/gen/openapi"
	"gmc/api/internal/inventory"
	"gmc/api/internal/platform"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func parsePagination(pageParam, pageSizeParam *int, defaultSize, maxSize int) (int, int, int) {
	page := 1
	if pageParam != nil && *pageParam >= 1 {
		page = *pageParam
	}
	pageSize := defaultSize
	if pageSizeParam != nil && *pageSizeParam >= 1 && *pageSizeParam <= maxSize {
		pageSize = *pageSizeParam
	}
	offset := (page - 1) * pageSize
	return page, pageSize, offset
}

func calcTotalPages(total, pageSize int) int {
	if total <= 0 || pageSize <= 0 {
		return 1
	}
	return int(math.Ceil(float64(total) / float64(pageSize)))
}

// --- Inventory & Stock Ledger ---

func (s *Server) AdminListInventory(w http.ResponseWriter, r *http.Request, params openapi.AdminListInventoryParams) {
	_, user, ok := s.authSvc.RequirePermission(w, r, rbac.PermInventoryAdjust)
	if !ok {
		return
	}
	showFinance := rbac.HasPermission(user.Role, rbac.PermFinanceView)

	page, pageSize, offset := parsePagination(params.Page, params.PageSize, 25, 100)
	conds := []string{"p.deleted_at IS NULL", "pv.is_active = TRUE"}
	args := []any{}

	if params.Q != nil && strings.TrimSpace(*params.Q) != "" {
		args = append(args, "%"+strings.TrimSpace(*params.Q)+"%")
		conds = append(conds, fmt.Sprintf("(pv.sku ILIKE $%d OR p.name ILIKE $%d)", len(args), len(args)))
	}
	if params.State != nil {
		switch string(*params.State) {
		case "low", "low_stock":
			conds = append(conds, "pv.stock > 0 AND pv.stock <= pv.low_stock_threshold AND NOT pv.is_preorder")
		case "out", "out_of_stock":
			conds = append(conds, "pv.stock <= 0 AND NOT pv.is_preorder")
		case "preorder":
			conds = append(conds, "pv.is_preorder = TRUE")
		}
	}

	whereClause := strings.Join(conds, " AND ")
	var total int
	_ = s.pool.QueryRow(r.Context(), "SELECT COUNT(*)::int FROM product_variants pv JOIN products p ON p.id = pv.product_id WHERE "+whereClause, args...).Scan(&total)

	argsWithPage := append(append([]any{}, args...), pageSize, offset)
	query := fmt.Sprintf(`
		SELECT pv.id, pv.sku, pv.price_idr, pv.cost_idr, pv.stock, pv.low_stock_threshold, pv.is_preorder,
		       p.id, p.name, p.slug
		FROM product_variants pv
		JOIN products p ON p.id = pv.product_id
		WHERE %s
		ORDER BY pv.stock ASC, pv.sku ASC
		LIMIT $%d OFFSET $%d
	`, whereClause, len(args)+1, len(args)+2)

	rows, err := s.pool.Query(r.Context(), query, argsWithPage...)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	defer rows.Close()

	items := make([]openapi.AdminInventoryItem, 0)
	for rows.Next() {
		var vID, pID uuid.UUID
		var sku, pName, pSlug string
		var priceIdr int64
		var costIdr *int64
		var stock, lowThresh int32
		var isPreorder bool
		if err := rows.Scan(&vID, &sku, &priceIdr, &costIdr, &stock, &lowThresh, &isPreorder, &pID, &pName, &pSlug); err == nil {
			var costOut *int64
			if showFinance && costIdr != nil {
				costOut = costIdr
			}
			items = append(items, openapi.AdminInventoryItem{
				VariantId:         vID,
				ProductId:         pID,
				ProductName:       pName,
				ProductSlug:       pSlug,
				Sku:               sku,
				PriceIdr:          priceIdr,
				CostIdr:           costOut,
				Stock:             int(stock),
				LowStockThreshold: int(lowThresh),
				IsPreorder:        isPreorder,
				StockStatus:       inventory.ComputeStockStatus(stock, lowThresh, isPreorder),
			})
		}
	}

	platform.WritePrivateJSON(w, http.StatusOK, openapi.AdminInventoryListResponse{
		Items:      items,
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: calcTotalPages(total, pageSize),
	})
}

func (s *Server) AdminAdjustInventory(w http.ResponseWriter, r *http.Request) {
	_, user, ok := s.authSvc.RequirePermission(w, r, rbac.PermInventoryAdjust)
	if !ok {
		return
	}
	var req openapi.AdminInventoryAdjustRequest
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	if req.Delta == 0 {
		platform.WriteError(w, r, platform.NewAppError(http.StatusBadRequest, openapi.ValidationFailed, "Delta stok tidak boleh 0"))
		return
	}

	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	qtx := s.q.WithTx(tx)

	note := ""
	if req.Note != nil {
		note = strings.TrimSpace(*req.Note)
	}
	stockAfter, err := s.invSvc.Adjust(r.Context(), qtx, inventory.AdjustParams{
		VariantID: uuid.UUID(req.VariantId),
		Delta:     int32(req.Delta),
		Reason:    string(req.Reason),
		ActorID:   uuid.NullUUID{UUID: user.ID, Valid: true},
		Note:      note,
	})
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}

	_ = audit.Record(r.Context(), qtx, audit.Entry{
		ActorID:    uuid.NullUUID{UUID: user.ID, Valid: true},
		ActorEmail: user.Email,
		ActorRole:  user.Role,
		Action:     "inventory.adjust",
		EntityType: "variant",
		EntityID:   uuid.UUID(req.VariantId).String(),
		Summary:    fmt.Sprintf("Penyesuaian stok %+d (%s) → %d", req.Delta, req.Reason, stockAfter),
		After:      map[string]any{"stockAfter": stockAfter, "delta": req.Delta, "reason": req.Reason},
	})

	if err := tx.Commit(r.Context()); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	s.InvalidateCatalogCache()

	var sku, pName, pSlug string
	var pID uuid.UUID
	var priceIdr int64
	var lowThresh int32
	var isPreorder bool
	_ = s.pool.QueryRow(r.Context(), `
		SELECT pv.sku, pv.price_idr, pv.low_stock_threshold, pv.is_preorder, p.id, p.name, p.slug
		FROM product_variants pv JOIN products p ON p.id = pv.product_id WHERE pv.id = $1
	`, uuid.UUID(req.VariantId)).Scan(&sku, &priceIdr, &lowThresh, &isPreorder, &pID, &pName, &pSlug)

	platform.WritePrivateJSON(w, http.StatusOK, openapi.AdminInventoryItem{
		VariantId:         req.VariantId,
		ProductId:         pID,
		ProductName:       pName,
		ProductSlug:       pSlug,
		Sku:               sku,
		PriceIdr:          priceIdr,
		Stock:             int(stockAfter),
		LowStockThreshold: int(lowThresh),
		IsPreorder:        isPreorder,
		StockStatus:       inventory.ComputeStockStatus(stockAfter, lowThresh, isPreorder),
	})
}

func (s *Server) AdminRestockInventory(w http.ResponseWriter, r *http.Request) {
	_, user, ok := s.authSvc.RequirePermission(w, r, rbac.PermInventoryAdjust)
	if !ok {
		return
	}
	var req openapi.AdminInventoryRestockRequest
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	canEditCost := rbac.HasPermission(user.Role, rbac.PermFinanceView)

	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	qtx := s.q.WithTx(tx)

	note := "Restock barang masuk"
	if req.SupplierNote != nil && strings.TrimSpace(*req.SupplierNote) != "" {
		note = strings.TrimSpace(*req.SupplierNote)
	}

	processed := 0
	for _, line := range req.Lines {
		if line.Quantity <= 0 {
			continue
		}
		var vID uuid.UUID
		if err := tx.QueryRow(r.Context(), `SELECT id FROM product_variants WHERE LOWER(sku) = LOWER($1)`, strings.TrimSpace(line.Sku)).Scan(&vID); err != nil {
			platform.WriteError(w, r, platform.NewAppError(http.StatusBadRequest, openapi.ValidationFailed, fmt.Sprintf("SKU tidak ditemukan: %s", line.Sku)))
			return
		}
		if _, err := s.invSvc.Adjust(r.Context(), qtx, inventory.AdjustParams{
			VariantID: vID,
			Delta:     int32(line.Quantity),
			Reason:    "restock",
			ActorID:   uuid.NullUUID{UUID: user.ID, Valid: true},
			Note:      note,
		}); err != nil {
			platform.WriteError(w, r, err)
			return
		}
		if canEditCost && line.CostIdr != nil && *line.CostIdr >= 0 {
			_, _ = tx.Exec(r.Context(), `UPDATE product_variants SET cost_idr = $2 WHERE id = $1`, vID, *line.CostIdr)
		}
		processed++
	}

	_ = audit.Record(r.Context(), qtx, audit.Entry{
		ActorID:    uuid.NullUUID{UUID: user.ID, Valid: true},
		ActorEmail: user.Email,
		ActorRole:  user.Role,
		Action:     "inventory.restock",
		EntityType: "inventory",
		EntityID:   "batch",
		Summary:    fmt.Sprintf("Restock %d baris SKU (%s)", processed, note),
		After:      req,
	})

	if err := tx.Commit(r.Context()); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	s.InvalidateCatalogCache()
	platform.WritePrivateJSON(w, http.StatusOK, map[string]any{"processed": processed})
}

func (s *Server) AdminStockOpname(w http.ResponseWriter, r *http.Request) {
	_, user, ok := s.authSvc.RequirePermission(w, r, rbac.PermInventoryAdjust)
	if !ok {
		return
	}
	var req openapi.AdminInventoryOpnameRequest
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

	note := "Stock Opname"
	if req.Note != nil && strings.TrimSpace(*req.Note) != "" {
		note = strings.TrimSpace(*req.Note)
	}

	adjusted := 0
	for _, line := range req.Lines {
		if line.CountedStock < 0 {
			continue
		}
		var vID uuid.UUID
		var currentStock int32
		if err := tx.QueryRow(r.Context(), `SELECT id, stock FROM product_variants WHERE LOWER(sku) = LOWER($1) FOR UPDATE`, strings.TrimSpace(line.Sku)).Scan(&vID, &currentStock); err != nil {
			platform.WriteError(w, r, platform.NewAppError(http.StatusBadRequest, openapi.ValidationFailed, fmt.Sprintf("SKU tidak ditemukan: %s", line.Sku)))
			return
		}
		delta := int32(line.CountedStock) - currentStock
		if delta == 0 {
			continue
		}
		if _, err := s.invSvc.Adjust(r.Context(), qtx, inventory.AdjustParams{
			VariantID: vID,
			Delta:     delta,
			Reason:    "correction",
			ActorID:   uuid.NullUUID{UUID: user.ID, Valid: true},
			Note:      note,
		}); err != nil {
			platform.WriteError(w, r, err)
			return
		}
		adjusted++
	}

	_ = audit.Record(r.Context(), qtx, audit.Entry{
		ActorID:    uuid.NullUUID{UUID: user.ID, Valid: true},
		ActorEmail: user.Email,
		ActorRole:  user.Role,
		Action:     "inventory.opname",
		EntityType: "inventory",
		EntityID:   "opname",
		Summary:    fmt.Sprintf("Stock opname menyesuaikan %d SKU", adjusted),
		After:      req,
	})

	if err := tx.Commit(r.Context()); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	s.InvalidateCatalogCache()
	platform.WritePrivateJSON(w, http.StatusOK, map[string]any{"adjusted": adjusted})
}

func (s *Server) AdminListStockMovements(w http.ResponseWriter, r *http.Request, params openapi.AdminListStockMovementsParams) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermInventoryAdjust); !ok {
		return
	}
	page, pageSize, offset := parsePagination(params.Page, params.PageSize, 25, 100)
	conds := []string{"1=1"}
	args := []any{}

	if params.Sku != nil && strings.TrimSpace(*params.Sku) != "" {
		args = append(args, "%"+strings.TrimSpace(*params.Sku)+"%")
		conds = append(conds, fmt.Sprintf("pv.sku ILIKE $%d", len(args)))
	}
	if params.Reason != nil && strings.TrimSpace(*params.Reason) != "" {
		args = append(args, strings.TrimSpace(*params.Reason))
		conds = append(conds, fmt.Sprintf("sm.reason = $%d", len(args)))
	}

	whereClause := strings.Join(conds, " AND ")
	var total int
	_ = s.pool.QueryRow(r.Context(), `
		SELECT COUNT(*)::int FROM stock_movements sm
		JOIN product_variants pv ON pv.id = sm.variant_id
		WHERE `+whereClause, args...).Scan(&total)

	argsWithPage := append(append([]any{}, args...), pageSize, offset)
	query := fmt.Sprintf(`
		SELECT sm.id, sm.variant_id, pv.sku, p.name, sm.delta, sm.stock_after, sm.reason, u.email, sm.note, sm.created_at
		FROM stock_movements sm
		JOIN product_variants pv ON pv.id = sm.variant_id
		JOIN products p ON p.id = pv.product_id
		LEFT JOIN users u ON u.id = sm.actor_id
		WHERE %s
		ORDER BY sm.created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, len(args)+1, len(args)+2)

	rows, err := s.pool.Query(r.Context(), query, argsWithPage...)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	defer rows.Close()

	items := make([]openapi.AdminStockMovement, 0)
	for rows.Next() {
		var mID, vID uuid.UUID
		var sku, pName, reason string
		var delta, stockAfter int32
		var actorEmail, note *string
		var createdAt time.Time
		if err := rows.Scan(&mID, &vID, &sku, &pName, &delta, &stockAfter, &reason, &actorEmail, &note, &createdAt); err == nil {
			items = append(items, openapi.AdminStockMovement{
				Id:          mID,
				VariantId:   vID,
				Sku:         sku,
				ProductName: pName,
				Delta:       int(delta),
				StockAfter:  int(stockAfter),
				Reason:      reason,
				ActorEmail:  actorEmail,
				Note:        note,
				CreatedAt:   createdAt,
			})
		}
	}

	platform.WritePrivateJSON(w, http.StatusOK, openapi.AdminStockMovementListResponse{
		Items:      items,
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: calcTotalPages(total, pageSize),
	})
}

func (s *Server) AdminExportStockMovementsCsv(w http.ResponseWriter, r *http.Request, params openapi.AdminExportStockMovementsCsvParams) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermInventoryAdjust); !ok {
		return
	}
	var delim *string
	if params.Delimiter != nil {
		d := string(*params.Delimiter)
		delim = &d
	}
	rows, err := s.pool.Query(r.Context(), `
		SELECT sm.created_at, pv.sku, p.name, sm.delta, sm.stock_after, sm.reason, COALESCE(u.email, 'system'), COALESCE(sm.note, '')
		FROM stock_movements sm
		JOIN product_variants pv ON pv.id = sm.variant_id
		JOIN products p ON p.id = pv.product_id
		LEFT JOIN users u ON u.id = sm.actor_id
		ORDER BY sm.created_at DESC
		LIMIT 5000
	`)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	defer rows.Close()

	headers := []string{"created_at", "sku", "product_name", "delta", "stock_after", "reason", "actor_email", "note"}
	var csvRows [][]string
	for rows.Next() {
		var createdAt time.Time
		var sku, pName, reason, actor, note string
		var delta, after int32
		if err := rows.Scan(&createdAt, &sku, &pName, &delta, &after, &reason, &actor, &note); err == nil {
			csvRows = append(csvRows, []string{
				createdAt.Format(time.RFC3339),
				sku, pName, fmt.Sprintf("%d", delta), fmt.Sprintf("%d", after), reason, actor, note,
			})
		}
	}
	writeCSVDownload(w, r, "gmc-stock-movements.csv", delim, headers, csvRows)
}

func (s *Server) AdminGetInventoryValuation(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermFinanceView); !ok {
		return
	}
	var totalUnits, missingCost int
	var totalRetail, totalCost int64
	err := s.pool.QueryRow(r.Context(), `
		SELECT
			COALESCE(SUM(pv.stock), 0)::int AS total_units,
			COALESCE(SUM(pv.stock * pv.price_idr), 0)::bigint AS total_retail_value_idr,
			COALESCE(SUM(pv.stock * COALESCE(pv.cost_idr, 0)), 0)::bigint AS total_cost_value_idr,
			COUNT(*) FILTER (WHERE pv.cost_idr IS NULL)::int AS missing_cost_variants
		FROM product_variants pv
		JOIN products p ON p.id = pv.product_id
		WHERE p.deleted_at IS NULL AND pv.is_active = TRUE
	`).Scan(&totalUnits, &totalRetail, &totalCost, &missingCost)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	platform.WritePrivateJSON(w, http.StatusOK, openapi.AdminInventoryValuation{
		TotalCostValueIdr:   totalCost,
		TotalRetailValueIdr: totalRetail,
		TotalUnits:          totalUnits,
		MissingCostVariants: missingCost,
	})
}

// --- Customers ---

func (s *Server) AdminListCustomers(w http.ResponseWriter, r *http.Request, params openapi.AdminListCustomersParams) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermCustomersRead); !ok {
		return
	}
	page, pageSize, offset := parsePagination(params.Page, params.PageSize, 20, 100)
	conds := []string{"u.role = 'customer'"}
	args := []any{}

	if params.Q != nil && strings.TrimSpace(*params.Q) != "" {
		args = append(args, "%"+strings.TrimSpace(*params.Q)+"%")
		conds = append(conds, fmt.Sprintf("(u.name ILIKE $%d OR u.email ILIKE $%d OR u.phone ILIKE $%d)", len(args), len(args), len(args)))
	}
	if params.IsActive != nil {
		args = append(args, *params.IsActive)
		conds = append(conds, fmt.Sprintf("u.is_active = $%d", len(args)))
	}
	havingClause := ""
	if params.HasOrders != nil {
		if *params.HasOrders {
			havingClause = "HAVING COUNT(o.id) > 0"
		} else {
			havingClause = "HAVING COUNT(o.id) = 0"
		}
	}

	whereClause := strings.Join(conds, " AND ")
	var total int
	_ = s.pool.QueryRow(r.Context(), fmt.Sprintf(`
		SELECT COUNT(*)::int FROM (
			SELECT u.id FROM users u LEFT JOIN orders o ON o.user_id = u.id
			WHERE %s GROUP BY u.id %s
		) sub
	`, whereClause, havingClause), args...).Scan(&total)

	argsWithPage := append(append([]any{}, args...), pageSize, offset)
	query := fmt.Sprintf(`
		SELECT u.id, u.name, u.email, u.phone, u.is_active, u.created_at,
		       COUNT(o.id)::int AS order_count,
		       COALESCE(SUM(o.total_idr) FILTER (WHERE o.status <> 'dibatalkan'), 0)::bigint AS total_spent_idr,
		       MAX(o.created_at) AS last_order_at
		FROM users u
		LEFT JOIN orders o ON o.user_id = u.id
		WHERE %s
		GROUP BY u.id
		%s
		ORDER BY u.created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, havingClause, len(args)+1, len(args)+2)

	rows, err := s.pool.Query(r.Context(), query, argsWithPage...)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	defer rows.Close()

	items := make([]openapi.AdminCustomerRow, 0)
	for rows.Next() {
		var id uuid.UUID
		var name, email, phone string
		var isActive bool
		var createdAt time.Time
		var orderCount int
		var totalSpent int64
		var lastOrder *time.Time
		if err := rows.Scan(&id, &name, &email, &phone, &isActive, &createdAt, &orderCount, &totalSpent, &lastOrder); err == nil {
			items = append(items, openapi.AdminCustomerRow{
				Id:            id,
				Name:          name,
				Email:         email,
				Phone:         phone,
				IsActive:      isActive,
				CreatedAt:     createdAt,
				OrderCount:    orderCount,
				TotalSpentIdr: totalSpent,
				LastOrderAt:   lastOrder,
			})
		}
	}

	platform.WritePrivateJSON(w, http.StatusOK, openapi.AdminCustomerListResponse{
		Items:      items,
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: calcTotalPages(total, pageSize),
	})
}

func (s *Server) AdminExportCustomersCsv(w http.ResponseWriter, r *http.Request, params openapi.AdminExportCustomersCsvParams) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermCustomersRead); !ok {
		return
	}
	var delim *string
	if params.Delimiter != nil {
		d := string(*params.Delimiter)
		delim = &d
	}
	rows, err := s.pool.Query(r.Context(), `
		SELECT u.name, u.email, u.phone, u.is_active, u.created_at,
		       COUNT(o.id)::int AS order_count,
		       COALESCE(SUM(o.total_idr) FILTER (WHERE o.status <> 'dibatalkan'), 0)::bigint AS total_spent_idr
		FROM users u
		LEFT JOIN orders o ON o.user_id = u.id
		WHERE u.role = 'customer'
		GROUP BY u.id
		ORDER BY u.created_at DESC
	`)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	defer rows.Close()

	headers := []string{"name", "email", "phone", "is_active", "order_count", "total_spent_idr", "created_at"}
	var csvRows [][]string
	for rows.Next() {
		var name, email, phone string
		var isActive bool
		var createdAt time.Time
		var orderCount int
		var totalSpent int64
		if err := rows.Scan(&name, &email, &phone, &isActive, &createdAt, &orderCount, &totalSpent); err == nil {
			csvRows = append(csvRows, []string{
				name, email, phone, fmt.Sprintf("%t", isActive), fmt.Sprintf("%d", orderCount), fmt.Sprintf("%d", totalSpent), createdAt.Format(time.RFC3339),
			})
		}
	}
	writeCSVDownload(w, r, "gmc-customers.csv", delim, headers, csvRows)
}

func (s *Server) AdminGetCustomer(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermCustomersRead); !ok {
		return
	}
	u, err := s.q.GetUserByID(r.Context(), uuid.UUID(id))
	if err != nil || u.Role != "customer" {
		platform.WriteError(w, r, platform.NewAppError(http.StatusNotFound, openapi.NotFound, "Pelanggan tidak ditemukan"))
		return
	}

	addrs, _ := s.q.ListUserAddresses(r.Context(), u.ID)
	apiAddrs := make([]openapi.Address, 0, len(addrs))
	for _, a := range addrs {
		apiAddrs = append(apiAddrs, openapi.Address{
			Id:            a.ID,
			Label:         a.Label,
			RecipientName: a.RecipientName,
			Phone:         a.Phone,
			Street:        a.Street,
			City:          a.City,
			Province:      a.Province,
			PostalCode:    a.PostalCode,
			IsDefault:     a.IsDefault,
		})
	}

	var orderCount int
	var totalSpent int64
	_ = s.pool.QueryRow(r.Context(), `
		SELECT COUNT(*)::int, COALESCE(SUM(total_idr) FILTER (WHERE status <> 'dibatalkan'), 0)::bigint
		FROM orders WHERE user_id = $1
	`, u.ID).Scan(&orderCount, &totalSpent)
	var aov int64
	if orderCount > 0 {
		aov = totalSpent / int64(orderCount)
	}

	orderRows, err := s.pool.Query(r.Context(), `
		SELECT o.order_no, o.status, o.payment_status, o.total_idr, o.created_at,
		       COALESCE((SELECT SUM(quantity) FROM order_items WHERE order_id = o.id), 0)::int,
		       COALESCE((SELECT name FROM order_items WHERE order_id = o.id LIMIT 1), ''),
		       (SELECT image_url FROM order_items WHERE order_id = o.id LIMIT 1)
		FROM orders o
		WHERE o.user_id = $1
		ORDER BY o.created_at DESC
		LIMIT 50
	`, u.ID)
	ordersList := make([]openapi.OrderSummary, 0)
	if err == nil {
		defer orderRows.Close()
		for orderRows.Next() {
			var orderNo, status, payStatus, firstName string
			var totalIdr int64
			var createdAt time.Time
			var itemCount int
			var firstImg *string
			if err := orderRows.Scan(&orderNo, &status, &payStatus, &totalIdr, &createdAt, &itemCount, &firstName, &firstImg); err == nil {
				ordersList = append(ordersList, openapi.OrderSummary{
					OrderNo:           orderNo,
					Status:            openapi.OrderStatus(status),
					PaymentStatus:     openapi.PaymentStatus(payStatus),
					TotalIdr:          totalIdr,
					CreatedAt:         createdAt,
					ItemCount:         itemCount,
					FirstItemName:     firstName,
					FirstItemImageUrl: firstImg,
				})
			}
		}
	}

	platform.WritePrivateJSON(w, http.StatusOK, openapi.AdminCustomerDetail{
		Id:            u.ID,
		Name:          u.Name,
		Email:         u.Email,
		Phone:         u.Phone,
		IsActive:      u.IsActive,
		InternalNote:  u.InternalNote,
		CreatedAt:     u.CreatedAt,
		OrderCount:    orderCount,
		TotalSpentIdr: totalSpent,
		AovIdr:        aov,
		Addresses:     apiAddrs,
		Orders:        ordersList,
	})
}

func (s *Server) AdminUpdateCustomer(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermCustomersWrite); !ok {
		return
	}
	var req openapi.AdminCustomerPatchRequest
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	u, err := s.q.GetUserByID(r.Context(), uuid.UUID(id))
	if err != nil {
		platform.WriteError(w, r, platform.NewAppError(http.StatusNotFound, openapi.NotFound, "Pelanggan tidak ditemukan"))
		return
	}
	name := u.Name
	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		name = strings.TrimSpace(*req.Name)
	}
	phone := u.Phone
	if req.Phone != nil && strings.TrimSpace(*req.Phone) != "" {
		phone = platform.NormalizePhoneOrKeep(*req.Phone)
	}
	note := u.InternalNote
	if req.InternalNote != nil {
		note = req.InternalNote
	}

	_, err = s.pool.Exec(r.Context(), `
		UPDATE users SET name = $2, phone = $3, internal_note = $4, updated_at = now() WHERE id = $1
	`, uuid.UUID(id), name, phone, note)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	s.AdminGetCustomer(w, r, id)
}

func (s *Server) AdminDeactivateCustomer(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	_, user, ok := s.authSvc.RequirePermission(w, r, rbac.PermCustomersWrite)
	if !ok {
		return
	}
	_, _ = s.pool.Exec(r.Context(), `UPDATE users SET is_active = FALSE, updated_at = now() WHERE id = $1 AND role = 'customer'`, uuid.UUID(id))
	_ = s.q.RevokeAllUserSessions(r.Context(), uuid.UUID(id))
	_ = audit.Record(r.Context(), s.q, audit.Entry{
		ActorID:    uuid.NullUUID{UUID: user.ID, Valid: true},
		ActorEmail: user.Email,
		ActorRole:  user.Role,
		Action:     "customer.deactivate",
		EntityType: "customer",
		EntityID:   uuid.UUID(id).String(),
		Summary:    "Menonaktifkan akun pelanggan",
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) AdminReactivateCustomer(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermCustomersWrite); !ok {
		return
	}
	_, _ = s.pool.Exec(r.Context(), `UPDATE users SET is_active = TRUE, updated_at = now() WHERE id = $1 AND role = 'customer'`, uuid.UUID(id))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) AdminForceCustomerPasswordReset(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermCustomersWrite); !ok {
		return
	}
	_, _ = s.pool.Exec(r.Context(), `UPDATE users SET must_change_password = TRUE, updated_at = now() WHERE id = $1 AND role = 'customer'`, uuid.UUID(id))
	_ = s.q.RevokeAllUserSessions(r.Context(), uuid.UUID(id))
	w.WriteHeader(http.StatusNoContent)
}

// --- Vouchers ---

func (s *Server) scanVoucherRows(rows interface {
	Next() bool
	Scan(dest ...any) error
}) []openapi.AdminVoucher {
	items := make([]openapi.AdminVoucher, 0)
	for rows.Next() {
		var v db.Voucher
		if err := rows.Scan(
			&v.ID, &v.Code, &v.Description, &v.Kind, &v.Value,
			&v.MinSubtotalIdr, &v.MaxDiscountIdr, &v.ScopeCategoryIds, &v.ScopeBrandIds,
			&v.StartsAt, &v.EndsAt, &v.UsageLimit, &v.PerUserLimit, &v.UsedCount,
			&v.IsActive, &v.Version, &v.CreatedAt,
		); err == nil {
			items = append(items, mapDBVoucherRow(v))
		}
	}
	return items
}

func (s *Server) AdminListVouchers(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermVouchersManage); !ok {
		return
	}
	rows, err := s.pool.Query(r.Context(), `
		SELECT id, code, description, kind, value, min_subtotal_idr, max_discount_idr,
		       scope_category_ids, scope_brand_ids, starts_at, ends_at, usage_limit,
		       per_user_limit, used_count, is_active, version, created_at
		FROM vouchers
		ORDER BY created_at DESC
	`)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	defer rows.Close()
	items := s.scanVoucherRows(rows)
	platform.WritePrivateJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) AdminCreateVoucher(w http.ResponseWriter, r *http.Request) {
	_, user, ok := s.authSvc.RequirePermission(w, r, rbac.PermVouchersManage)
	if !ok {
		return
	}
	var req openapi.AdminVoucherInput
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	code := strings.ToUpper(strings.TrimSpace(req.Code))
	if code == "" {
		platform.WriteError(w, r, platform.NewAppError(http.StatusBadRequest, openapi.ValidationFailed, "Kode voucher wajib diisi"))
		return
	}
	var usageLim *int32
	if req.UsageLimit != nil {
		ul := int32(*req.UsageLimit)
		usageLim = &ul
	}
	catIDs := make([]uuid.UUID, 0)
	brandIDs := make([]uuid.UUID, 0)
	if req.ScopeCategoryIds != nil {
		for _, cid := range *req.ScopeCategoryIds {
			catIDs = append(catIDs, uuid.UUID(cid))
		}
	}
	if req.ScopeBrandIds != nil {
		for _, bid := range *req.ScopeBrandIds {
			brandIDs = append(brandIDs, uuid.UUID(bid))
		}
	}

	var v db.Voucher
	err := s.pool.QueryRow(r.Context(), `
		INSERT INTO vouchers (
			id, code, description, kind, value, min_subtotal_idr, max_discount_idr,
			scope_category_ids, scope_brand_ids, starts_at, ends_at, usage_limit,
			per_user_limit, is_active
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id, code, description, kind, value, min_subtotal_idr, max_discount_idr,
		          scope_category_ids, scope_brand_ids, starts_at, ends_at, usage_limit,
		          per_user_limit, used_count, is_active, version, created_at
	`, platform.NewUUID(), code, req.Description, string(req.Kind), int32(req.Value),
		req.MinSubtotalIdr, req.MaxDiscountIdr, catIDs, brandIDs, req.StartsAt, req.EndsAt,
		usageLim, int32(req.PerUserLimit), req.IsActive,
	).Scan(
		&v.ID, &v.Code, &v.Description, &v.Kind, &v.Value,
		&v.MinSubtotalIdr, &v.MaxDiscountIdr, &v.ScopeCategoryIds, &v.ScopeBrandIds,
		&v.StartsAt, &v.EndsAt, &v.UsageLimit, &v.PerUserLimit, &v.UsedCount,
		&v.IsActive, &v.Version, &v.CreatedAt,
	)
	if err != nil {
		platform.WriteError(w, r, platform.NewAppError(http.StatusConflict, openapi.Conflict, "Kode voucher sudah digunakan"))
		return
	}
	_ = audit.Record(r.Context(), s.q, audit.Entry{
		ActorID:    uuid.NullUUID{UUID: user.ID, Valid: true},
		ActorEmail: user.Email,
		ActorRole:  user.Role,
		Action:     "voucher.create",
		EntityType: "voucher",
		EntityID:   v.ID.String(),
		Summary:    fmt.Sprintf("Membuat voucher %s", code),
		After:      req,
	})
	platform.WritePrivateJSON(w, http.StatusCreated, mapDBVoucherRow(v))
}

func (s *Server) AdminUpdateVoucher(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	_, user, ok := s.authSvc.RequirePermission(w, r, rbac.PermVouchersManage)
	if !ok {
		return
	}
	var req openapi.AdminVoucherInput
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	code := strings.ToUpper(strings.TrimSpace(req.Code))
	var usageLim *int32
	if req.UsageLimit != nil {
		ul := int32(*req.UsageLimit)
		usageLim = &ul
	}
	catIDs := make([]uuid.UUID, 0)
	brandIDs := make([]uuid.UUID, 0)
	if req.ScopeCategoryIds != nil {
		for _, cid := range *req.ScopeCategoryIds {
			catIDs = append(catIDs, uuid.UUID(cid))
		}
	}
	if req.ScopeBrandIds != nil {
		for _, bid := range *req.ScopeBrandIds {
			brandIDs = append(brandIDs, uuid.UUID(bid))
		}
	}
	ver := int32(0)
	if req.Version != nil {
		ver = int32(*req.Version)
	}

	var v db.Voucher
	err := s.pool.QueryRow(r.Context(), `
		UPDATE vouchers
		SET code = $2, description = $3, kind = $4, value = $5,
		    min_subtotal_idr = $6, max_discount_idr = $7,
		    scope_category_ids = $8, scope_brand_ids = $9,
		    starts_at = $10, ends_at = $11, usage_limit = $12,
		    per_user_limit = $13, is_active = $14,
		    version = version + 1
		WHERE id = $1 AND ($15 <= 0 OR version = $15)
		RETURNING id, code, description, kind, value, min_subtotal_idr, max_discount_idr,
		          scope_category_ids, scope_brand_ids, starts_at, ends_at, usage_limit,
		          per_user_limit, used_count, is_active, version, created_at
	`, uuid.UUID(id), code, req.Description, string(req.Kind), int32(req.Value),
		req.MinSubtotalIdr, req.MaxDiscountIdr, catIDs, brandIDs,
		req.StartsAt, req.EndsAt, usageLim, int32(req.PerUserLimit), req.IsActive, ver,
	).Scan(
		&v.ID, &v.Code, &v.Description, &v.Kind, &v.Value,
		&v.MinSubtotalIdr, &v.MaxDiscountIdr, &v.ScopeCategoryIds, &v.ScopeBrandIds,
		&v.StartsAt, &v.EndsAt, &v.UsageLimit, &v.PerUserLimit, &v.UsedCount,
		&v.IsActive, &v.Version, &v.CreatedAt,
	)
	if err != nil {
		platform.WriteError(w, r, platform.NewAppError(http.StatusConflict, openapi.Conflict, "Voucher tidak ditemukan atau telah diubah oleh pengguna lain"))
		return
	}
	_ = audit.Record(r.Context(), s.q, audit.Entry{
		ActorID:    uuid.NullUUID{UUID: user.ID, Valid: true},
		ActorEmail: user.Email,
		ActorRole:  user.Role,
		Action:     "voucher.update",
		EntityType: "voucher",
		EntityID:   v.ID.String(),
		Summary:    fmt.Sprintf("Memperbarui voucher %s", code),
		After:      req,
	})
	platform.WritePrivateJSON(w, http.StatusOK, mapDBVoucherRow(v))
}

func (s *Server) AdminDeleteVoucher(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermVouchersManage); !ok {
		return
	}
	_, _ = s.pool.Exec(r.Context(), `DELETE FROM vouchers WHERE id = $1`, uuid.UUID(id))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) AdminListVoucherRedemptions(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermVouchersManage); !ok {
		return
	}
	rows, err := s.pool.Query(r.Context(), `
		SELECT vr.id, o.order_no, vr.phone, vr.discount_idr, vr.created_at
		FROM voucher_redemptions vr
		JOIN orders o ON o.id = vr.order_id
		WHERE vr.voucher_id = $1
		ORDER BY vr.created_at DESC
	`, uuid.UUID(id))
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	defer rows.Close()
	items := make([]openapi.AdminVoucherRedemption, 0)
	for rows.Next() {
		var rID uuid.UUID
		var orderNo string
		var phone *string
		var disc int64
		var createdAt time.Time
		if rows.Scan(&rID, &orderNo, &phone, &disc, &createdAt) == nil {
			items = append(items, openapi.AdminVoucherRedemption{
				Id:            rID,
				OrderNo:       orderNo,
				CustomerPhone: phone,
				DiscountIdr:   disc,
				CreatedAt:     createdAt,
			})
		}
	}
	platform.WritePrivateJSON(w, http.StatusOK, map[string]any{"items": items})
}

func mapDBVoucherRow(v db.Voucher) openapi.AdminVoucher {
	var uLim *int
	if v.UsageLimit != nil {
		ul := int(*v.UsageLimit)
		uLim = &ul
	}
	cats := v.ScopeCategoryIds
	brands := v.ScopeBrandIds
	return openapi.AdminVoucher{
		Id:               v.ID,
		Code:             v.Code,
		Description:      v.Description,
		Kind:             openapi.AdminVoucherKind(v.Kind),
		Value:            int(v.Value),
		MinSubtotalIdr:   v.MinSubtotalIdr,
		MaxDiscountIdr:   v.MaxDiscountIdr,
		ScopeCategoryIds: &cats,
		ScopeBrandIds:    &brands,
		StartsAt:         v.StartsAt,
		EndsAt:           v.EndsAt,
		UsageLimit:       uLim,
		PerUserLimit:     int(v.PerUserLimit),
		UsedCount:        int(v.UsedCount),
		IsActive:         v.IsActive,
		Version:          int(v.Version),
		CreatedAt:        v.CreatedAt,
	}
}

// --- Content: Announcements, Slides, FAQs ---

func (s *Server) AdminListAnnouncements(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermContentManage); !ok {
		return
	}
	rows, _ := s.q.ListAllAnnouncements(r.Context())
	items := make([]openapi.AdminAnnouncement, 0, len(rows))
	for _, a := range rows {
		items = append(items, openapi.AdminAnnouncement{
			Id:        a.ID,
			Text:      a.Text,
			LinkUrl:   a.LinkUrl,
			SortOrder: int(a.SortOrder),
			IsActive:  a.IsActive,
			StartsAt:  a.StartsAt,
			EndsAt:    a.EndsAt,
		})
	}
	platform.WritePrivateJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) AdminCreateAnnouncement(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermContentManage); !ok {
		return
	}
	var req openapi.AdminAnnouncementInput
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	sortOrd := int32(0)
	if req.SortOrder != nil {
		sortOrd = int32(*req.SortOrder)
	}
	a, err := s.q.CreateAnnouncement(r.Context(), db.CreateAnnouncementParams{
		ID:        platform.NewUUID(),
		Text:      req.Text,
		LinkUrl:   req.LinkUrl,
		SortOrder: sortOrd,
		IsActive:  req.IsActive,
		StartsAt:  req.StartsAt,
		EndsAt:    req.EndsAt,
	})
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	s.InvalidateSiteCache()
	platform.WritePrivateJSON(w, http.StatusCreated, openapi.AdminAnnouncement{
		Id:        a.ID,
		Text:      a.Text,
		LinkUrl:   a.LinkUrl,
		SortOrder: int(a.SortOrder),
		IsActive:  a.IsActive,
		StartsAt:  a.StartsAt,
		EndsAt:    a.EndsAt,
	})
}

func (s *Server) AdminUpdateAnnouncement(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermContentManage); !ok {
		return
	}
	var req openapi.AdminAnnouncementInput
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	sortOrd := int32(0)
	if req.SortOrder != nil {
		sortOrd = int32(*req.SortOrder)
	}
	a, err := s.q.UpdateAnnouncement(r.Context(), db.UpdateAnnouncementParams{
		ID:        uuid.UUID(id),
		Text:      req.Text,
		LinkUrl:   req.LinkUrl,
		SortOrder: sortOrd,
		IsActive:  req.IsActive,
		StartsAt:  req.StartsAt,
		EndsAt:    req.EndsAt,
	})
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	s.InvalidateSiteCache()
	platform.WritePrivateJSON(w, http.StatusOK, openapi.AdminAnnouncement{
		Id:        a.ID,
		Text:      a.Text,
		LinkUrl:   a.LinkUrl,
		SortOrder: int(a.SortOrder),
		IsActive:  a.IsActive,
		StartsAt:  a.StartsAt,
		EndsAt:    a.EndsAt,
	})
}

func (s *Server) AdminDeleteAnnouncement(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermContentManage); !ok {
		return
	}
	_ = s.q.DeleteAnnouncement(r.Context(), uuid.UUID(id))
	s.InvalidateSiteCache()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) AdminReorderAnnouncements(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermContentManage); !ok {
		return
	}
	var req openapi.ReorderRequest
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	for idx, id := range req.Ids {
		_, _ = s.pool.Exec(r.Context(), `UPDATE announcements SET sort_order = $2 WHERE id = $1`, uuid.UUID(id), idx)
	}
	s.InvalidateSiteCache()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) AdminListSlides(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermContentManage); !ok {
		return
	}
	rows, _ := s.q.ListAllPromoSlides(r.Context())
	items := make([]openapi.AdminPromoSlide, 0, len(rows))
	for _, sl := range rows {
		items = append(items, openapi.AdminPromoSlide{
			Id:             sl.ID,
			Label:          sl.Label,
			Headline:       sl.Headline,
			Body:           sl.Body,
			CtaLabel:       sl.CtaLabel,
			CtaUrl:         sl.CtaUrl,
			ImageUrl:       sl.ImageUrl,
			ImageMobileUrl: sl.ImageMobileUrl,
			SortOrder:      int(sl.SortOrder),
			IsActive:       sl.IsActive,
		})
	}
	platform.WritePrivateJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) AdminCreateSlide(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermContentManage); !ok {
		return
	}
	var req openapi.AdminPromoSlideInput
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	sortOrd := int32(0)
	if req.SortOrder != nil {
		sortOrd = int32(*req.SortOrder)
	}
	sl, err := s.q.CreatePromoSlide(r.Context(), db.CreatePromoSlideParams{
		ID:             platform.NewUUID(),
		Label:          req.Label,
		Headline:       req.Headline,
		Body:           req.Body,
		CtaLabel:       req.CtaLabel,
		CtaUrl:         req.CtaUrl,
		ImageUrl:       req.ImageUrl,
		ImageMobileUrl: req.ImageMobileUrl,
		SortOrder:      sortOrd,
		IsActive:       req.IsActive,
	})
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	s.InvalidateSiteCache()
	platform.WritePrivateJSON(w, http.StatusCreated, openapi.AdminPromoSlide{
		Id:             sl.ID,
		Label:          sl.Label,
		Headline:       sl.Headline,
		Body:           sl.Body,
		CtaLabel:       sl.CtaLabel,
		CtaUrl:         sl.CtaUrl,
		ImageUrl:       sl.ImageUrl,
		ImageMobileUrl: sl.ImageMobileUrl,
		SortOrder:      int(sl.SortOrder),
		IsActive:       sl.IsActive,
	})
}

func (s *Server) AdminUpdateSlide(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermContentManage); !ok {
		return
	}
	var req openapi.AdminPromoSlideInput
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	sortOrd := int32(0)
	if req.SortOrder != nil {
		sortOrd = int32(*req.SortOrder)
	}
	sl, err := s.q.UpdatePromoSlide(r.Context(), db.UpdatePromoSlideParams{
		ID:             uuid.UUID(id),
		Label:          req.Label,
		Headline:       req.Headline,
		Body:           req.Body,
		CtaLabel:       req.CtaLabel,
		CtaUrl:         req.CtaUrl,
		ImageUrl:       req.ImageUrl,
		ImageMobileUrl: req.ImageMobileUrl,
		SortOrder:      sortOrd,
		IsActive:       req.IsActive,
	})
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	s.InvalidateSiteCache()
	platform.WritePrivateJSON(w, http.StatusOK, openapi.AdminPromoSlide{
		Id:             sl.ID,
		Label:          sl.Label,
		Headline:       sl.Headline,
		Body:           sl.Body,
		CtaLabel:       sl.CtaLabel,
		CtaUrl:         sl.CtaUrl,
		ImageUrl:       sl.ImageUrl,
		ImageMobileUrl: sl.ImageMobileUrl,
		SortOrder:      int(sl.SortOrder),
		IsActive:       sl.IsActive,
	})
}

func (s *Server) AdminDeleteSlide(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermContentManage); !ok {
		return
	}
	_ = s.q.DeletePromoSlide(r.Context(), uuid.UUID(id))
	s.InvalidateSiteCache()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) AdminReorderSlides(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermContentManage); !ok {
		return
	}
	var req openapi.ReorderRequest
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	for idx, id := range req.Ids {
		_, _ = s.pool.Exec(r.Context(), `UPDATE promo_slides SET sort_order = $2 WHERE id = $1`, uuid.UUID(id), idx)
	}
	s.InvalidateSiteCache()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) AdminListFaqs(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermContentManage); !ok {
		return
	}
	rows, _ := s.q.ListAllFaqs(r.Context())
	items := make([]openapi.AdminFaq, 0, len(rows))
	for _, f := range rows {
		items = append(items, openapi.AdminFaq{
			Id:        f.ID,
			Anchor:    f.Anchor,
			Question:  f.Question,
			Answer:    f.Answer,
			SortOrder: int(f.SortOrder),
			IsActive:  f.IsActive,
		})
	}
	platform.WritePrivateJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) AdminCreateFaq(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermContentManage); !ok {
		return
	}
	var req openapi.AdminFaqInput
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	sortOrd := int32(0)
	if req.SortOrder != nil {
		sortOrd = int32(*req.SortOrder)
	}
	f, err := s.q.CreateFaq(r.Context(), db.CreateFaqParams{
		ID:        platform.NewUUID(),
		Anchor:    req.Anchor,
		Question:  req.Question,
		Answer:    req.Answer,
		SortOrder: sortOrd,
		IsActive:  req.IsActive,
	})
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	s.InvalidateSiteCache()
	platform.WritePrivateJSON(w, http.StatusCreated, openapi.AdminFaq{
		Id:        f.ID,
		Anchor:    f.Anchor,
		Question:  f.Question,
		Answer:    f.Answer,
		SortOrder: int(f.SortOrder),
		IsActive:  f.IsActive,
	})
}

func (s *Server) AdminUpdateFaq(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermContentManage); !ok {
		return
	}
	var req openapi.AdminFaqInput
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	sortOrd := int32(0)
	if req.SortOrder != nil {
		sortOrd = int32(*req.SortOrder)
	}
	f, err := s.q.UpdateFaq(r.Context(), db.UpdateFaqParams{
		ID:        uuid.UUID(id),
		Anchor:    req.Anchor,
		Question:  req.Question,
		Answer:    req.Answer,
		SortOrder: sortOrd,
		IsActive:  req.IsActive,
	})
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	s.InvalidateSiteCache()
	platform.WritePrivateJSON(w, http.StatusOK, openapi.AdminFaq{
		Id:        f.ID,
		Anchor:    f.Anchor,
		Question:  f.Question,
		Answer:    f.Answer,
		SortOrder: int(f.SortOrder),
		IsActive:  f.IsActive,
	})
}

func (s *Server) AdminDeleteFaq(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermContentManage); !ok {
		return
	}
	_ = s.q.DeleteFaq(r.Context(), uuid.UUID(id))
	s.InvalidateSiteCache()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) AdminReorderFaqs(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermContentManage); !ok {
		return
	}
	var req openapi.ReorderRequest
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	for idx, id := range req.Ids {
		_, _ = s.pool.Exec(r.Context(), `UPDATE faqs SET sort_order = $2 WHERE id = $1`, uuid.UUID(id), idx)
	}
	s.InvalidateSiteCache()
	w.WriteHeader(http.StatusNoContent)
}

// --- Contact Messages & Subscribers ---

func (s *Server) AdminListMessages(w http.ResponseWriter, r *http.Request, params openapi.AdminListMessagesParams) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermMessagesManage); !ok {
		return
	}
	page, pageSize, offset := parsePagination(params.Page, params.PageSize, 20, 100)
	conds := []string{"1=1"}
	args := []any{}

	if params.Status != nil && string(*params.Status) != "" {
		args = append(args, string(*params.Status))
		conds = append(conds, fmt.Sprintf("status = $%d", len(args)))
	}
	if params.Q != nil && strings.TrimSpace(*params.Q) != "" {
		args = append(args, "%"+strings.TrimSpace(*params.Q)+"%")
		conds = append(conds, fmt.Sprintf("(name ILIKE $%d OR email ILIKE $%d OR message ILIKE $%d)", len(args), len(args), len(args)))
	}
	whereClause := strings.Join(conds, " AND ")

	var total int
	_ = s.pool.QueryRow(r.Context(), "SELECT COUNT(*)::int FROM contact_messages WHERE "+whereClause, args...).Scan(&total)

	argsWithPage := append(append([]any{}, args...), pageSize, offset)
	query := fmt.Sprintf(`
		SELECT id, name, phone, email, subject, message, status, internal_note, created_at
		FROM contact_messages
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, len(args)+1, len(args)+2)

	rows, err := s.pool.Query(r.Context(), query, argsWithPage...)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	defer rows.Close()

	items := make([]openapi.AdminMessage, 0)
	for rows.Next() {
		var mID uuid.UUID
		var name, phone, email, subject, message, status string
		var internalNote *string
		var createdAt time.Time
		if err := rows.Scan(&mID, &name, &phone, &email, &subject, &message, &status, &internalNote, &createdAt); err == nil {
			items = append(items, openapi.AdminMessage{
				Id:           mID,
				Name:         name,
				Phone:        phone,
				Email:        email,
				Subject:      subject,
				Message:      message,
				Status:       openapi.AdminMessageStatus(status),
				InternalNote: internalNote,
				CreatedAt:    createdAt,
			})
		}
	}
	platform.WritePrivateJSON(w, http.StatusOK, openapi.AdminMessageListResponse{
		Items:      items,
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: calcTotalPages(total, pageSize),
	})
}

func (s *Server) AdminUpdateMessage(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	_, user, ok := s.authSvc.RequirePermission(w, r, rbac.PermMessagesManage)
	if !ok {
		return
	}
	var req openapi.AdminMessagePatchRequest
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	status := "read"
	if req.Status != nil {
		status = string(*req.Status)
	}
	var mID uuid.UUID
	var name, phone, email, subject, message, outStatus string
	var internalNote *string
	var createdAt time.Time
	err := s.pool.QueryRow(r.Context(), `
		UPDATE contact_messages
		SET status = $2,
		    internal_note = COALESCE($3, internal_note),
		    handled_by = $4,
		    handled_at = now()
		WHERE id = $1
		RETURNING id, name, phone, email, subject, message, status, internal_note, created_at
	`, uuid.UUID(id), status, req.InternalNote, user.ID).Scan(
		&mID, &name, &phone, &email, &subject, &message, &outStatus, &internalNote, &createdAt,
	)
	if err != nil {
		platform.WriteError(w, r, platform.NewAppError(http.StatusNotFound, openapi.NotFound, "Pesan tidak ditemukan"))
		return
	}
	platform.WritePrivateJSON(w, http.StatusOK, openapi.AdminMessage{
		Id:           mID,
		Name:         name,
		Phone:        phone,
		Email:        email,
		Subject:      subject,
		Message:      message,
		Status:       openapi.AdminMessageStatus(outStatus),
		InternalNote: internalNote,
		CreatedAt:    createdAt,
	})
}

func (s *Server) AdminDeleteMessage(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermMessagesManage); !ok {
		return
	}
	_, _ = s.pool.Exec(r.Context(), `DELETE FROM contact_messages WHERE id = $1`, uuid.UUID(id))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) AdminListSubscribers(w http.ResponseWriter, r *http.Request, params openapi.AdminListSubscribersParams) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermMessagesManage); !ok {
		return
	}
	page, pageSize, offset := parsePagination(params.Page, params.PageSize, 25, 100)
	conds := []string{"1=1"}
	args := []any{}
	if params.Q != nil && strings.TrimSpace(*params.Q) != "" {
		args = append(args, "%"+strings.TrimSpace(*params.Q)+"%")
		conds = append(conds, fmt.Sprintf("(email ILIKE $%d OR name ILIKE $%d)", len(args), len(args)))
	}
	whereClause := strings.Join(conds, " AND ")

	var total int
	_ = s.pool.QueryRow(r.Context(), "SELECT COUNT(*)::int FROM newsletter_subscribers WHERE "+whereClause, args...).Scan(&total)

	argsWithPage := append(append([]any{}, args...), pageSize, offset)
	rows, err := s.pool.Query(r.Context(), fmt.Sprintf(`
		SELECT id, email, name, created_at FROM newsletter_subscribers
		WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d
	`, whereClause, len(args)+1, len(args)+2), argsWithPage...)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	defer rows.Close()

	items := make([]openapi.AdminSubscriber, 0)
	for rows.Next() {
		var id uuid.UUID
		var email string
		var name *string
		var createdAt time.Time
		if err := rows.Scan(&id, &email, &name, &createdAt); err == nil {
			items = append(items, openapi.AdminSubscriber{
				Id:        id,
				Email:     email,
				Name:      name,
				CreatedAt: createdAt,
			})
		}
	}
	platform.WritePrivateJSON(w, http.StatusOK, openapi.AdminSubscriberListResponse{
		Items:      items,
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: calcTotalPages(total, pageSize),
	})
}

func (s *Server) AdminExportSubscribersCsv(w http.ResponseWriter, r *http.Request, params openapi.AdminExportSubscribersCsvParams) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermMessagesManage); !ok {
		return
	}
	var delim *string
	if params.Delimiter != nil {
		d := string(*params.Delimiter)
		delim = &d
	}
	rows, err := s.pool.Query(r.Context(), `SELECT email, COALESCE(name, ''), created_at FROM newsletter_subscribers ORDER BY created_at DESC`)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	defer rows.Close()
	headers := []string{"email", "name", "created_at"}
	var csvRows [][]string
	for rows.Next() {
		var email, name string
		var createdAt time.Time
		if rows.Scan(&email, &name, &createdAt) == nil {
			csvRows = append(csvRows, []string{email, name, createdAt.Format(time.RFC3339)})
		}
	}
	writeCSVDownload(w, r, "gmc-newsletter-subscribers.csv", delim, headers, csvRows)
}

func (s *Server) AdminDeleteSubscriber(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermMessagesManage); !ok {
		return
	}
	_, _ = s.pool.Exec(r.Context(), `DELETE FROM newsletter_subscribers WHERE id = $1`, uuid.UUID(id))
	w.WriteHeader(http.StatusNoContent)
}

// --- Analytical Reports (10 built-in reports + CSV export) ---

func (s *Server) runAnalyticalReport(r *http.Request, name string, fromStr, toStr *string) ([]string, []map[string]any, [][]string) {
	now := time.Now()
	from := now.AddDate(0, 0, -30)
	to := now
	if fromStr != nil {
		if t, err := time.Parse("2006-01-02", *fromStr); err == nil {
			from = t
		}
	}
	if toStr != nil {
		if t, err := time.Parse("2006-01-02", *toStr); err == nil {
			to = t.Add(24*time.Hour - time.Nanosecond)
		}
	}

	var cols []string
	var rowsOut []map[string]any
	var csvRows [][]string

	switch name {
	case "daily_sales", "sales_by_day":
		cols = []string{"date", "orders", "confirmed_revenue_idr", "discount_idr", "shipping_idr"}
		rows, err := s.pool.Query(r.Context(), `
			SELECT to_char(created_at AT TIME ZONE 'Asia/Jakarta', 'YYYY-MM-DD') AS dt,
			       COUNT(*)::int,
			       COALESCE(SUM(total_idr) FILTER (WHERE status IN ('diproses','dikirim','siap_diambil','selesai')), 0)::bigint,
			       COALESCE(SUM(discount_idr), 0)::bigint,
			       COALESCE(SUM(shipping_idr), 0)::bigint
			FROM orders WHERE created_at >= $1 AND created_at <= $2
			GROUP BY dt ORDER BY dt DESC
		`, from, to)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var dt string
				var ord int
				var rev, disc, ship int64
				if rows.Scan(&dt, &ord, &rev, &disc, &ship) == nil {
					rowsOut = append(rowsOut, map[string]any{"date": dt, "orders": ord, "confirmed_revenue_idr": rev, "discount_idr": disc, "shipping_idr": ship})
					csvRows = append(csvRows, []string{dt, fmt.Sprintf("%d", ord), fmt.Sprintf("%d", rev), fmt.Sprintf("%d", disc), fmt.Sprintf("%d", ship)})
				}
			}
		}
	case "by_product", "sales_by_product":
		cols = []string{"sku", "product_name", "brand", "units_sold", "revenue_idr"}
		rows, err := s.pool.Query(r.Context(), `
			SELECT oi.sku, oi.name, oi.brand,
			       COALESCE(SUM(oi.quantity), 0)::int AS units,
			       COALESCE(SUM(oi.line_total_idr), 0)::bigint AS rev
			FROM order_items oi JOIN orders o ON o.id = oi.order_id
			WHERE o.created_at >= $1 AND o.created_at <= $2 AND o.status <> 'dibatalkan'
			GROUP BY oi.sku, oi.name, oi.brand
			ORDER BY rev DESC LIMIT 100
		`, from, to)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var sku, pName, brand string
				var units int
				var rev int64
				if rows.Scan(&sku, &pName, &brand, &units, &rev) == nil {
					rowsOut = append(rowsOut, map[string]any{"sku": sku, "product_name": pName, "brand": brand, "units_sold": units, "revenue_idr": rev})
					csvRows = append(csvRows, []string{sku, pName, brand, fmt.Sprintf("%d", units), fmt.Sprintf("%d", rev)})
				}
			}
		}
	case "by_category", "sales_by_category":
		cols = []string{"category", "units_sold", "revenue_idr"}
		rows, err := s.pool.Query(r.Context(), `
			SELECT c.name, COALESCE(SUM(oi.quantity), 0)::int AS units, COALESCE(SUM(oi.line_total_idr), 0)::bigint AS rev
			FROM order_items oi
			JOIN orders o ON o.id = oi.order_id
			JOIN product_variants pv ON pv.id = oi.variant_id
			JOIN products p ON p.id = pv.product_id
			JOIN categories c ON c.id = p.category_id
			WHERE o.created_at >= $1 AND o.created_at <= $2 AND o.status <> 'dibatalkan'
			GROUP BY c.name ORDER BY rev DESC
		`, from, to)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var cat string
				var units int
				var rev int64
				if rows.Scan(&cat, &units, &rev) == nil {
					rowsOut = append(rowsOut, map[string]any{"category": cat, "units_sold": units, "revenue_idr": rev})
					csvRows = append(csvRows, []string{cat, fmt.Sprintf("%d", units), fmt.Sprintf("%d", rev)})
				}
			}
		}
	case "by_brand", "sales_by_brand":
		cols = []string{"brand", "units_sold", "revenue_idr"}
		rows, err := s.pool.Query(r.Context(), `
			SELECT oi.brand, COALESCE(SUM(oi.quantity), 0)::int AS units, COALESCE(SUM(oi.line_total_idr), 0)::bigint AS rev
			FROM order_items oi JOIN orders o ON o.id = oi.order_id
			WHERE o.created_at >= $1 AND o.created_at <= $2 AND o.status <> 'dibatalkan'
			GROUP BY oi.brand ORDER BY rev DESC
		`, from, to)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var brand string
				var units int
				var rev int64
				if rows.Scan(&brand, &units, &rev) == nil {
					rowsOut = append(rowsOut, map[string]any{"brand": brand, "units_sold": units, "revenue_idr": rev})
					csvRows = append(csvRows, []string{brand, fmt.Sprintf("%d", units), fmt.Sprintf("%d", rev)})
				}
			}
		}
	case "by_payment_delivery", "sales_by_channel_payment":
		cols = []string{"source", "delivery_method", "payment_method", "orders", "revenue_idr"}
		rows, err := s.pool.Query(r.Context(), `
			SELECT source, delivery_method, payment_method, COUNT(*)::int, COALESCE(SUM(total_idr), 0)::bigint
			FROM orders WHERE created_at >= $1 AND created_at <= $2 AND status <> 'dibatalkan'
			GROUP BY source, delivery_method, payment_method ORDER BY 5 DESC
		`, from, to)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var src, deliv, pay string
				var ord int
				var rev int64
				if rows.Scan(&src, &deliv, &pay, &ord, &rev) == nil {
					rowsOut = append(rowsOut, map[string]any{"source": src, "delivery_method": deliv, "payment_method": pay, "orders": ord, "revenue_idr": rev})
					csvRows = append(csvRows, []string{src, deliv, pay, fmt.Sprintf("%d", ord), fmt.Sprintf("%d", rev)})
				}
			}
		}
	case "cancelled_orders":
		cols = []string{"order_no", "customer_name", "total_idr", "cancel_reason", "created_at"}
		rows, err := s.pool.Query(r.Context(), `
			SELECT order_no, customer_name, total_idr, COALESCE(cancel_reason, '-'), created_at
			FROM orders WHERE created_at >= $1 AND created_at <= $2 AND status = 'dibatalkan'
			ORDER BY created_at DESC LIMIT 100
		`, from, to)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var orderNo, cName, reason string
				var totalIdr int64
				var createdAt time.Time
				if rows.Scan(&orderNo, &cName, &totalIdr, &reason, &createdAt) == nil {
					rowsOut = append(rowsOut, map[string]any{"order_no": orderNo, "customer_name": cName, "total_idr": totalIdr, "cancel_reason": reason, "created_at": createdAt.Format(time.RFC3339)})
					csvRows = append(csvRows, []string{orderNo, cName, fmt.Sprintf("%d", totalIdr), reason, createdAt.Format(time.RFC3339)})
				}
			}
		}
	case "dead_stock":
		cols = []string{"sku", "product_name", "stock", "price_idr", "retail_value_idr"}
		rows, err := s.pool.Query(r.Context(), `
			SELECT pv.sku, p.name, pv.stock, pv.price_idr, (pv.stock * pv.price_idr)::bigint
			FROM product_variants pv JOIN products p ON p.id = pv.product_id
			WHERE p.deleted_at IS NULL AND pv.stock > 0
			  AND NOT EXISTS (
				SELECT 1 FROM order_items oi JOIN orders o ON o.id = oi.order_id
				WHERE oi.variant_id = pv.id AND o.created_at >= $1 AND o.status <> 'dibatalkan'
			  )
			ORDER BY 5 DESC LIMIT 100
		`, from)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var sku, pName string
				var stock int32
				var price, val int64
				if rows.Scan(&sku, &pName, &stock, &price, &val) == nil {
					rowsOut = append(rowsOut, map[string]any{"sku": sku, "product_name": pName, "stock": int(stock), "price_idr": price, "retail_value_idr": val})
					csvRows = append(csvRows, []string{sku, pName, fmt.Sprintf("%d", stock), fmt.Sprintf("%d", price), fmt.Sprintf("%d", val)})
				}
			}
		}
	case "stock_movements":
		cols = []string{"reason", "movements_count", "net_delta"}
		rows, err := s.pool.Query(r.Context(), `
			SELECT reason, COUNT(*)::int, COALESCE(SUM(delta), 0)::int
			FROM stock_movements WHERE created_at >= $1 AND created_at <= $2
			GROUP BY reason ORDER BY 2 DESC
		`, from, to)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var reason string
				var cnt, net int
				if rows.Scan(&reason, &cnt, &net) == nil {
					rowsOut = append(rowsOut, map[string]any{"reason": reason, "movements_count": cnt, "net_delta": net})
					csvRows = append(csvRows, []string{reason, fmt.Sprintf("%d", cnt), fmt.Sprintf("%d", net)})
				}
			}
		}
	case "top_customers":
		cols = []string{"customer_name", "customer_phone", "customer_email", "orders", "total_spent_idr"}
		rows, err := s.pool.Query(r.Context(), `
			SELECT customer_name, customer_phone, customer_email, COUNT(*)::int, COALESCE(SUM(total_idr), 0)::bigint
			FROM orders WHERE created_at >= $1 AND created_at <= $2 AND status <> 'dibatalkan'
			GROUP BY customer_name, customer_phone, customer_email
			ORDER BY 5 DESC LIMIT 50
		`, from, to)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var cName, cPhone, cEmail string
				var ord int
				var spent int64
				if rows.Scan(&cName, &cPhone, &cEmail, &ord, &spent) == nil {
					rowsOut = append(rowsOut, map[string]any{"customer_name": cName, "customer_phone": cPhone, "customer_email": cEmail, "orders": ord, "total_spent_idr": spent})
					csvRows = append(csvRows, []string{cName, cPhone, cEmail, fmt.Sprintf("%d", ord), fmt.Sprintf("%d", spent)})
				}
			}
		}
	default: // vouchers
		cols = []string{"code", "redemptions", "total_discount_idr", "attributed_revenue_idr"}
		rows, err := s.pool.Query(r.Context(), `
			SELECT v.code, COUNT(vr.id)::int, COALESCE(SUM(vr.discount_idr), 0)::bigint, COALESCE(SUM(o.total_idr), 0)::bigint
			FROM vouchers v
			LEFT JOIN voucher_redemptions vr ON vr.voucher_id = v.id
			LEFT JOIN orders o ON o.id = vr.order_id AND o.status <> 'dibatalkan'
			GROUP BY v.code ORDER BY 4 DESC
		`)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var code string
				var red int
				var disc, rev int64
				if rows.Scan(&code, &red, &disc, &rev) == nil {
					rowsOut = append(rowsOut, map[string]any{"code": code, "redemptions": red, "total_discount_idr": disc, "attributed_revenue_idr": rev})
					csvRows = append(csvRows, []string{code, fmt.Sprintf("%d", red), fmt.Sprintf("%d", disc), fmt.Sprintf("%d", rev)})
				}
			}
		}
	}

	if rowsOut == nil {
		rowsOut = []map[string]any{}
	}
	return cols, rowsOut, csvRows
}

func (s *Server) AdminGetReport(w http.ResponseWriter, r *http.Request, name openapi.AdminGetReportParamsName, params openapi.AdminGetReportParams) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermReportsView); !ok {
		return
	}
	cols, rowsOut, _ := s.runAnalyticalReport(r, string(name), params.From, params.To)
	platform.WritePrivateJSON(w, http.StatusOK, openapi.AdminReportResponse{
		Name:    string(name),
		Columns: cols,
		Rows:    rowsOut,
	})
}

func (s *Server) AdminExportReportCsv(w http.ResponseWriter, r *http.Request, name string, params openapi.AdminExportReportCsvParams) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermReportsView); !ok {
		return
	}
	var delim *string
	if params.Delimiter != nil {
		d := string(*params.Delimiter)
		delim = &d
	}
	cols, _, csvRows := s.runAnalyticalReport(r, name, params.From, params.To)
	writeCSVDownload(w, r, fmt.Sprintf("gmc-report-%s.csv", name), delim, cols, csvRows)
}

// --- Staff Management ---

func (s *Server) countActiveAdmins(r *http.Request) int {
	var cnt int
	_ = s.pool.QueryRow(r.Context(), `SELECT COUNT(*)::int FROM users WHERE role = 'admin' AND is_active = TRUE`).Scan(&cnt)
	return cnt
}

func (s *Server) AdminListStaff(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermStaffManage); !ok {
		return
	}
	rows, err := s.pool.Query(r.Context(), `
		SELECT u.id, u.name, u.email, u.phone, u.role, u.is_active,
		       (ut.confirmed_at IS NOT NULL) AS totp_enabled,
		       u.last_login_at, u.created_at
		FROM users u
		LEFT JOIN user_totp ut ON ut.user_id = u.id
		WHERE u.role IN ('admin', 'staff')
		ORDER BY u.role ASC, u.name ASC
	`)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	defer rows.Close()
	items := make([]openapi.AdminStaffUser, 0)
	for rows.Next() {
		var uID uuid.UUID
		var name, email, phone, role string
		var isActive, totpEnabled bool
		var lastLoginAt *time.Time
		var createdAt time.Time
		if rows.Scan(&uID, &name, &email, &phone, &role, &isActive, &totpEnabled, &lastLoginAt, &createdAt) == nil {
			items = append(items, openapi.AdminStaffUser{
				Id:          uID,
				Name:        name,
				Email:       email,
				Phone:       phone,
				Role:        openapi.UserRole(role),
				IsActive:    isActive,
				TotpEnabled: totpEnabled,
				LastLoginAt: lastLoginAt,
				CreatedAt:   createdAt,
			})
		}
	}
	platform.WritePrivateJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) AdminCreateStaff(w http.ResponseWriter, r *http.Request) {
	_, actorUser, ok := s.authSvc.RequirePermission(w, r, rbac.PermStaffManage)
	if !ok {
		return
	}
	var req openapi.AdminCreateStaffRequest
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	randBytes := make([]byte, 6)
	_, _ = rand.Read(randBytes)
	tempPassword := "Gmc-" + hex.EncodeToString(randBytes) + "!"
	hash, err := platform.HashPassword(tempPassword)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}

	id := platform.NewUUID()
	email := strings.ToLower(strings.TrimSpace(string(req.Email)))
	phone := platform.NormalizePhoneOrKeep(req.Phone)
	role := string(req.Role)
	if role != "admin" && role != "staff" {
		role = "staff"
	}

	u, err := s.q.CreateUser(r.Context(), db.CreateUserParams{
		ID:                 id,
		Email:              email,
		Name:               strings.TrimSpace(req.Name),
		Phone:              phone,
		PasswordHash:       hash,
		Role:               role,
		IsActive:           true,
		MustChangePassword: true,
	})
	if err != nil {
		platform.WriteError(w, r, platform.NewAppError(http.StatusConflict, openapi.EmailTaken, "Email sudah terdaftar"))
		return
	}

	_ = audit.Record(r.Context(), s.q, audit.Entry{
		ActorID:    uuid.NullUUID{UUID: actorUser.ID, Valid: true},
		ActorEmail: actorUser.Email,
		ActorRole:  actorUser.Role,
		Action:     "staff.create",
		EntityType: "staff",
		EntityID:   id.String(),
		Summary:    fmt.Sprintf("Menambahkan akun %s (%s)", email, role),
		After:      req,
	})

	platform.WritePrivateJSON(w, http.StatusCreated, openapi.AdminCreateStaffResponse{
		TemporaryPassword: tempPassword,
		User: openapi.AdminStaffUser{
			Id:          u.ID,
			Name:        u.Name,
			Email:       u.Email,
			Phone:       u.Phone,
			Role:        openapi.UserRole(u.Role),
			IsActive:    u.IsActive,
			TotpEnabled: false,
			CreatedAt:   u.CreatedAt,
		},
	})
}

func (s *Server) AdminUpdateStaff(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	_, actorUser, ok := s.authSvc.RequirePermission(w, r, rbac.PermStaffManage)
	if !ok {
		return
	}
	var req openapi.AdminUpdateStaffRequest
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	u, err := s.q.GetUserByID(r.Context(), uuid.UUID(id))
	if err != nil {
		platform.WriteError(w, r, platform.NewAppError(http.StatusNotFound, openapi.NotFound, "Staf tidak ditemukan"))
		return
	}
	name := u.Name
	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		name = strings.TrimSpace(*req.Name)
	}
	phone := u.Phone
	if req.Phone != nil && strings.TrimSpace(*req.Phone) != "" {
		phone = platform.NormalizePhoneOrKeep(*req.Phone)
	}
	role := u.Role
	if req.Role != nil {
		if u.Role == "admin" && string(*req.Role) != "admin" {
			if s.countActiveAdmins(r) <= 1 {
				platform.WriteError(w, r, platform.NewAppError(http.StatusConflict, openapi.Conflict, "Tidak dapat menurunkan role admin aktif terakhir"))
				return
			}
		}
		role = string(*req.Role)
	}

	_, _ = s.pool.Exec(r.Context(), `UPDATE users SET name = $2, phone = $3, role = $4, updated_at = now() WHERE id = $1`, uuid.UUID(id), name, phone, role)
	_ = audit.Record(r.Context(), s.q, audit.Entry{
		ActorID:    uuid.NullUUID{UUID: actorUser.ID, Valid: true},
		ActorEmail: actorUser.Email,
		ActorRole:  actorUser.Role,
		Action:     "staff.update",
		EntityType: "staff",
		EntityID:   uuid.UUID(id).String(),
		Summary:    fmt.Sprintf("Memperbarui profil/role staf %s", u.Email),
		After:      req,
	})

	totpRec, totpErr := s.q.GetUserTotp(r.Context(), uuid.UUID(id))
	totpEnabled := totpErr == nil && totpRec.ConfirmedAt != nil
	platform.WritePrivateJSON(w, http.StatusOK, openapi.AdminStaffUser{
		Id:          id,
		Name:        name,
		Email:       u.Email,
		Phone:       phone,
		Role:        openapi.UserRole(role),
		IsActive:    u.IsActive,
		TotpEnabled: totpEnabled,
		CreatedAt:   u.CreatedAt,
	})
}

func (s *Server) AdminDeactivateStaff(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	_, actorUser, ok := s.authSvc.RequirePermission(w, r, rbac.PermStaffManage)
	if !ok {
		return
	}
	if actorUser.ID == uuid.UUID(id) {
		platform.WriteError(w, r, platform.NewAppError(http.StatusConflict, openapi.Conflict, "Tidak dapat menonaktifkan akun Anda sendiri"))
		return
	}
	u, err := s.q.GetUserByID(r.Context(), uuid.UUID(id))
	if err != nil {
		platform.WriteError(w, r, platform.NewAppError(http.StatusNotFound, openapi.NotFound, "Staf tidak ditemukan"))
		return
	}
	if u.Role == "admin" && u.IsActive {
		if s.countActiveAdmins(r) <= 1 {
			platform.WriteError(w, r, platform.NewAppError(http.StatusConflict, openapi.Conflict, "Tidak dapat menonaktifkan admin aktif terakhir"))
			return
		}
	}
	_, _ = s.pool.Exec(r.Context(), `UPDATE users SET is_active = FALSE, updated_at = now() WHERE id = $1`, uuid.UUID(id))
	_ = s.q.RevokeAllUserSessions(r.Context(), uuid.UUID(id))
	_ = audit.Record(r.Context(), s.q, audit.Entry{
		ActorID:    uuid.NullUUID{UUID: actorUser.ID, Valid: true},
		ActorEmail: actorUser.Email,
		ActorRole:  actorUser.Role,
		Action:     "staff.deactivate",
		EntityType: "staff",
		EntityID:   uuid.UUID(id).String(),
		Summary:    fmt.Sprintf("Menonaktifkan akun staf %s", u.Email),
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) AdminReactivateStaff(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermStaffManage); !ok {
		return
	}
	_, _ = s.pool.Exec(r.Context(), `UPDATE users SET is_active = TRUE, updated_at = now() WHERE id = $1`, uuid.UUID(id))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) AdminForceLogoutStaff(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermStaffManage); !ok {
		return
	}
	_ = s.q.RevokeAllUserSessions(r.Context(), uuid.UUID(id))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) AdminResetStaff2fa(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	_, actorUser, ok := s.authSvc.RequirePermission(w, r, rbac.PermStaffManage)
	if !ok {
		return
	}
	_, _ = s.pool.Exec(r.Context(), `DELETE FROM user_totp WHERE user_id = $1`, uuid.UUID(id))
	_ = audit.Record(r.Context(), s.q, audit.Entry{
		ActorID:    uuid.NullUUID{UUID: actorUser.ID, Valid: true},
		ActorEmail: actorUser.Email,
		ActorRole:  actorUser.Role,
		Action:     "staff.reset_2fa",
		EntityType: "staff",
		EntityID:   uuid.UUID(id).String(),
		Summary:    "Mereset 2FA staf",
	})
	w.WriteHeader(http.StatusNoContent)
}

// --- Audit Log & Store Settings ---

func (s *Server) AdminListAuditLog(w http.ResponseWriter, r *http.Request, params openapi.AdminListAuditLogParams) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermAuditView); !ok {
		return
	}
	page, pageSize, offset := parsePagination(params.Page, params.PageSize, 25, 100)
	conds := []string{"1=1"}
	args := []any{}

	if params.ActorId != nil {
		args = append(args, uuid.UUID(*params.ActorId))
		conds = append(conds, fmt.Sprintf("actor_id = $%d", len(args)))
	}
	if params.EntityType != nil && strings.TrimSpace(*params.EntityType) != "" {
		args = append(args, strings.TrimSpace(*params.EntityType))
		conds = append(conds, fmt.Sprintf("entity_type = $%d", len(args)))
	}
	if params.EntityId != nil && strings.TrimSpace(*params.EntityId) != "" {
		args = append(args, strings.TrimSpace(*params.EntityId))
		conds = append(conds, fmt.Sprintf("entity_id = $%d", len(args)))
	}
	if params.Action != nil && strings.TrimSpace(*params.Action) != "" {
		args = append(args, strings.TrimSpace(*params.Action))
		conds = append(conds, fmt.Sprintf("action = $%d", len(args)))
	}
	whereClause := strings.Join(conds, " AND ")

	var total int
	_ = s.pool.QueryRow(r.Context(), "SELECT COUNT(*)::int FROM audit_log WHERE "+whereClause, args...).Scan(&total)

	argsWithPage := append(append([]any{}, args...), pageSize, offset)
	rows, err := s.pool.Query(r.Context(), fmt.Sprintf(`
		SELECT id, at, actor_id, actor_email, actor_role, action, entity_type, entity_id, summary, before, after, host(ip)
		FROM audit_log WHERE %s ORDER BY at DESC LIMIT $%d OFFSET $%d
	`, whereClause, len(args)+1, len(args)+2), argsWithPage...)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	defer rows.Close()

	items := make([]openapi.AdminAuditLogEntry, 0)
	for rows.Next() {
		var id uuid.UUID
		var at time.Time
		var actorID uuid.NullUUID
		var actorEmail, actorRole, action, entityType, summary string
		var entityID, ip *string
		var beforeBytes, afterBytes []byte
		if err := rows.Scan(&id, &at, &actorID, &actorEmail, &actorRole, &action, &entityType, &entityID, &summary, &beforeBytes, &afterBytes, &ip); err == nil {
			var actorPtr *openapi_types.UUID
			if actorID.Valid {
				u := openapi_types.UUID(actorID.UUID)
				actorPtr = &u
			}
			var beforeMap, afterMap *map[string]any
			if len(beforeBytes) > 0 {
				var m map[string]any
				if json.Unmarshal(beforeBytes, &m) == nil {
					beforeMap = &m
				}
			}
			if len(afterBytes) > 0 {
				var m map[string]any
				if json.Unmarshal(afterBytes, &m) == nil {
					afterMap = &m
				}
			}
			items = append(items, openapi.AdminAuditLogEntry{
				Id:         id,
				At:         at,
				ActorId:    actorPtr,
				ActorEmail: actorEmail,
				ActorRole:  actorRole,
				Action:     action,
				EntityType: entityType,
				EntityId:   entityID,
				Summary:    summary,
				Before:     beforeMap,
				After:      afterMap,
				Ip:         ip,
			})
		}
	}

	platform.WritePrivateJSON(w, http.StatusOK, openapi.AdminAuditLogListResponse{
		Items:      items,
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: calcTotalPages(total, pageSize),
	})
}

func (s *Server) AdminExportAuditLogCsv(w http.ResponseWriter, r *http.Request, params openapi.AdminExportAuditLogCsvParams) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermAuditView); !ok {
		return
	}
	var delim *string
	if params.Delimiter != nil {
		d := string(*params.Delimiter)
		delim = &d
	}
	rows, err := s.pool.Query(r.Context(), `
		SELECT at, actor_email, actor_role, action, entity_type, COALESCE(entity_id, ''), summary, COALESCE(host(ip), '')
		FROM audit_log ORDER BY at DESC LIMIT 5000
	`)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	defer rows.Close()

	headers := []string{"at", "actor_email", "actor_role", "action", "entity_type", "entity_id", "summary", "ip"}
	var csvRows [][]string
	for rows.Next() {
		var at time.Time
		var email, role, action, eType, eID, summary, ip string
		if rows.Scan(&at, &email, &role, &action, &eType, &eID, &summary, &ip) == nil {
			csvRows = append(csvRows, []string{at.Format(time.RFC3339), email, role, action, eType, eID, summary, ip})
		}
	}
	writeCSVDownload(w, r, "gmc-audit-log.csv", delim, headers, csvRows)
}

func (s *Server) AdminGetAllSettings(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermSettingsManage); !ok {
		return
	}
	rows, err := s.q.ListSiteSettings(r.Context())
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	groups := make([]openapi.AdminSettingsGroupPayload, 0, len(rows))
	for _, row := range rows {
		val := map[string]any{}
		_ = json.Unmarshal(row.Value, &val)
		groups = append(groups, openapi.AdminSettingsGroupPayload{
			Key:       row.Key,
			Value:     val,
			Version:   int(row.Version),
			UpdatedAt: row.UpdatedAt,
		})
	}
	platform.WritePrivateJSON(w, http.StatusOK, openapi.AdminSettingsAllResponse{Groups: groups})
}

func (s *Server) AdminGetSettingsGroup(w http.ResponseWriter, r *http.Request, group openapi.AdminGetSettingsGroupParamsGroup) {
	if _, _, ok := s.authSvc.RequirePermission(w, r, rbac.PermSettingsManage); !ok {
		return
	}
	row, err := s.q.GetSiteSetting(r.Context(), string(group))
	if err != nil {
		platform.WritePrivateJSON(w, http.StatusOK, openapi.AdminSettingsGroupPayload{
			Key:       string(group),
			Value:     map[string]any{},
			Version:   1,
			UpdatedAt: time.Now(),
		})
		return
	}
	val := map[string]any{}
	_ = json.Unmarshal(row.Value, &val)
	platform.WritePrivateJSON(w, http.StatusOK, openapi.AdminSettingsGroupPayload{
		Key:       row.Key,
		Value:     val,
		Version:   int(row.Version),
		UpdatedAt: row.UpdatedAt,
	})
}

func (s *Server) AdminUpdateSettingsGroup(w http.ResponseWriter, r *http.Request, group openapi.AdminUpdateSettingsGroupParamsGroup) {
	_, actorUser, ok := s.authSvc.RequirePermission(w, r, rbac.PermSettingsManage)
	if !ok {
		return
	}
	var req openapi.AdminSettingsGroupUpdateRequest
	if err := platform.DecodeStrictJSON(r, &req); err != nil {
		platform.WriteError(w, r, err)
		return
	}

	valBytes, _ := json.Marshal(req.Value)
	var newVer int32
	var updatedAt time.Time
	err := s.pool.QueryRow(r.Context(), `
		INSERT INTO site_settings (key, value, version, updated_by, updated_at)
		VALUES ($1, $2, 1, $3, now())
		ON CONFLICT (key) DO UPDATE
		SET value = EXCLUDED.value,
		    version = site_settings.version + 1,
		    updated_by = EXCLUDED.updated_by,
		    updated_at = now()
		WHERE site_settings.version = $4 OR $4 <= 0
		RETURNING version, updated_at
	`, string(group), valBytes, actorUser.ID, int32(req.Version)).Scan(&newVer, &updatedAt)
	if err != nil {
		platform.WriteError(w, r, platform.NewAppError(http.StatusConflict, openapi.Conflict, "Pengaturan telah diubah oleh pengguna lain. Muat ulang halaman."))
		return
	}

	_ = audit.Record(r.Context(), s.q, audit.Entry{
		ActorID:    uuid.NullUUID{UUID: actorUser.ID, Valid: true},
		ActorEmail: actorUser.Email,
		ActorRole:  actorUser.Role,
		Action:     "settings.update",
		EntityType: "settings",
		EntityID:   string(group),
		Summary:    fmt.Sprintf("Memperbarui pengaturan grup %s", group),
		After:      req.Value,
	})

	s.InvalidateSiteCache()
	platform.WritePrivateJSON(w, http.StatusOK, openapi.AdminSettingsGroupPayload{
		Key:       string(group),
		Value:     req.Value,
		Version:   int(newVer),
		UpdatedAt: updatedAt,
	})
}
