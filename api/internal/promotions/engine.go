package promotions

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type VoucherRule struct {
	ID                uuid.UUID
	Code              string
	Description       string
	Kind              string // "percent" | "fixed"
	Value             int32
	MaxDiscountIDR    *int64
	MinSubtotalIDR    int64
	UsageLimit        *int32
	PerUserLimit      int32
	UsedCount         int32
	StartsAt          *time.Time
	EndsAt            *time.Time
	IsActive          bool
	ScopeCategoryIDs  []uuid.UUID
	ScopeBrandIDs     []uuid.UUID
	CustomerUsedCount int64
}

type EligibleLine struct {
	CategoryID   uuid.UUID
	BrandID      uuid.UUID
	LineTotalIDR int64
}

type EvaluationResult struct {
	Valid       bool
	DiscountIDR int64
	Issue       string
}

// Evaluate computes the discount and validates all constraints for a voucher.
// Percent = floor(eligibleSubtotal * value / 100) capped by max_discount_idr and eligibleSubtotal.
// Fixed = min(value, eligibleSubtotal). Never makes total negative.
func Evaluate(v VoucherRule, lines []EligibleLine, now time.Time) EvaluationResult {
	if !v.IsActive {
		return EvaluationResult{Valid: false, Issue: "Kode voucher sudah tidak aktif"}
	}
	if v.StartsAt != nil && now.Before(*v.StartsAt) {
		return EvaluationResult{Valid: false, Issue: "Promo voucher belum dimulai"}
	}
	if v.EndsAt != nil && now.After(*v.EndsAt) {
		return EvaluationResult{Valid: false, Issue: "Kode voucher sudah kedaluwarsa"}
	}
	if v.UsageLimit != nil && v.UsedCount >= *v.UsageLimit {
		return EvaluationResult{Valid: false, Issue: "Kuota pemakaian voucher sudah habis"}
	}
	if v.PerUserLimit > 0 && v.CustomerUsedCount >= int64(v.PerUserLimit) {
		return EvaluationResult{Valid: false, Issue: "Batas pemakaian voucher untuk akun/nomor ini sudah tercapai"}
	}

	var totalSubtotal int64
	var eligibleSubtotal int64
	for _, l := range lines {
		totalSubtotal += l.LineTotalIDR
		if isLineEligible(l, v.ScopeCategoryIDs, v.ScopeBrandIDs) {
			eligibleSubtotal += l.LineTotalIDR
		}
	}

	if totalSubtotal < v.MinSubtotalIDR {
		return EvaluationResult{
			Valid: false,
			Issue: fmt.Sprintf("Minimum belanja untuk voucher %s adalah Rp %s", strings.ToUpper(v.Code), formatSimpleIDR(v.MinSubtotalIDR)),
		}
	}
	if eligibleSubtotal <= 0 {
		return EvaluationResult{
			Valid: false,
			Issue: "Tidak ada produk di keranjang yang memenuhi syarat kategori/brand voucher ini",
		}
	}

	var discount int64
	switch v.Kind {
	case "percent":
		discount = (eligibleSubtotal * int64(v.Value)) / 100
		if v.MaxDiscountIDR != nil && *v.MaxDiscountIDR > 0 && discount > *v.MaxDiscountIDR {
			discount = *v.MaxDiscountIDR
		}
	case "fixed":
		discount = int64(v.Value)
	default:
		return EvaluationResult{Valid: false, Issue: "Tipe voucher tidak valid"}
	}

	if discount > eligibleSubtotal {
		discount = eligibleSubtotal
	}
	if discount > totalSubtotal {
		discount = totalSubtotal
	}
	if discount < 0 {
		discount = 0
	}

	return EvaluationResult{
		Valid:       true,
		DiscountIDR: discount,
	}
}

func isLineEligible(l EligibleLine, scopeCats, scopeBrands []uuid.UUID) bool {
	if len(scopeCats) > 0 {
		catMatch := false
		for _, id := range scopeCats {
			if id == l.CategoryID {
				catMatch = true
				break
			}
		}
		if !catMatch {
			return false
		}
	}
	if len(scopeBrands) > 0 {
		brandMatch := false
		for _, id := range scopeBrands {
			if id == l.BrandID {
				brandMatch = true
				break
			}
		}
		if !brandMatch {
			return false
		}
	}
	return true
}

func formatSimpleIDR(amount int64) string {
	s := fmt.Sprintf("%d", amount)
	var out []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, byte(c))
	}
	return string(out)
}
