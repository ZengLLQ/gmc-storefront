package inventory

import (
	"context"
	"fmt"
	"net/http"

	"gmc/api/internal/gen/db"
	"gmc/api/internal/gen/openapi"
	"gmc/api/internal/platform"

	"github.com/google/uuid"
)

type AdjustParams struct {
	VariantID uuid.UUID
	Delta     int32
	Reason    string
	RefType   string
	RefID     uuid.NullUUID
	ActorID   uuid.NullUUID
	Note      string
}

type Service struct{}

func NewService() *Service {
	return &Service{}
}

// Adjust is the single source of truth for all stock modifications.
// It locks the variant row FOR UPDATE, enforces stock_after >= 0, updates variants.stock,
// and appends an immutable row to stock_movements.
func (s *Service) Adjust(ctx context.Context, q *db.Queries, p AdjustParams) (int32, error) {
	if p.Delta == 0 {
		v, err := q.GetVariantByID(ctx, p.VariantID)
		if err != nil {
			return 0, err
		}
		return v.Stock, nil
	}

	v, err := q.LockVariantForUpdate(ctx, p.VariantID)
	if err != nil {
		return 0, platform.NewAppError(http.StatusNotFound, openapi.NotFound, "Varian produk tidak ditemukan")
	}

	stockAfter := v.Stock + p.Delta
	if stockAfter < 0 {
		return 0, platform.NewAppError(
			http.StatusConflict,
			openapi.ErrorCodeOutOfStock,
			fmt.Sprintf("Stok tidak mencukupi untuk SKU %s (tersedia: %d)", v.Sku, v.Stock),
		)
	}

	if err := q.UpdateVariantStock(ctx, db.UpdateVariantStockParams{
		ID:    p.VariantID,
		Stock: stockAfter,
	}); err != nil {
		return 0, err
	}

	var refTypePtr *string
	if p.RefType != "" {
		refTypePtr = &p.RefType
	}
	var notePtr *string
	if p.Note != "" {
		notePtr = &p.Note
	}

	if _, err := q.InsertStockMovement(ctx, db.InsertStockMovementParams{
		ID:         platform.NewUUID(),
		VariantID:  p.VariantID,
		Delta:      p.Delta,
		StockAfter: stockAfter,
		Reason:     p.Reason,
		RefType:    refTypePtr,
		RefID:      p.RefID,
		Note:       notePtr,
		ActorID:    p.ActorID,
	}); err != nil {
		return 0, err
	}

	return stockAfter, nil
}

func ComputeStockStatus(stock int32, threshold int32, isPreorder bool) openapi.StockStatus {
	if isPreorder {
		return openapi.StockStatusPreorder
	}
	if stock <= 0 {
		return openapi.StockStatusOutOfStock
	}
	if threshold <= 0 {
		threshold = 3
	}
	if stock <= threshold {
		return openapi.StockStatusLowStock
	}
	return openapi.StockStatusInStock
}
