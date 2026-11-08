package promotions

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestVoucherEvaluation(t *testing.T) {
	now := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)
	catA := uuid.New()
	catB := uuid.New()
	brandX := uuid.New()

	lines := []EligibleLine{
		{CategoryID: catA, BrandID: brandX, LineTotalIDR: 3000000},
		{CategoryID: catB, BrandID: brandX, LineTotalIDR: 2500000},
	}

	// 1. Fixed discount valid
	res := Evaluate(VoucherRule{
		ID:             uuid.New(),
		Code:           "HEMAT100K",
		Kind:           "fixed",
		Value:          100000,
		MinSubtotalIDR: 2000000,
		PerUserLimit:   1,
		IsActive:       true,
	}, lines, now)
	if !res.Valid || res.DiscountIDR != 100000 {
		t.Fatalf("expected HEMAT100K valid with 100000 discount, got %+v", res)
	}

	// 2. Percent discount with max cap
	maxCap := int64(200000)
	res = Evaluate(VoucherRule{
		ID:             uuid.New(),
		Code:           "RAKITPC5",
		Kind:           "percent",
		Value:          5, // 5% of 5.5M = 275k, capped at 200k
		MaxDiscountIDR: &maxCap,
		MinSubtotalIDR: 5000000,
		PerUserLimit:   2,
		IsActive:       true,
	}, lines, now)
	if !res.Valid || res.DiscountIDR != 200000 {
		t.Fatalf("expected RAKITPC5 capped at 200000, got %+v", res)
	}

	// 3. Expired voucher rejected
	past := now.Add(-1 * time.Hour)
	res = Evaluate(VoucherRule{
		ID:       uuid.New(),
		Code:     "EXPIRED10",
		Kind:     "percent",
		Value:    10,
		EndsAt:   &past,
		IsActive: true,
	}, lines, now)
	if res.Valid {
		t.Fatalf("expected expired voucher to be invalid")
	}

	// 4. Category-scoped voucher applies only to catA (3M)
	res = Evaluate(VoucherRule{
		ID:               uuid.New(),
		Code:             "CATA10",
		Kind:             "percent",
		Value:            10,
		ScopeCategoryIDs: []uuid.UUID{catA},
		IsActive:         true,
	}, lines, now)
	if !res.Valid || res.DiscountIDR != 300000 {
		t.Fatalf("expected category-scoped discount 300000, got %+v", res)
	}
}
