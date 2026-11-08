package orders

import (
	"fmt"
	"strings"
	"time"
)

var allowedTransitions = map[string][]string{
	"menunggu_konfirmasi": {"menunggu_pembayaran", "diproses", "dibatalkan"},
	"menunggu_pembayaran": {"diproses", "dibatalkan"},
	"diproses":            {"dikirim", "siap_diambil", "dibatalkan"},
	"dikirim":             {"selesai"},
	"siap_diambil":        {"selesai"},
	"selesai":             {},
	"dibatalkan":          {},
}

func CanTransition(from, to string) bool {
	nexts, ok := allowedTransitions[from]
	if !ok {
		return false
	}
	for _, n := range nexts {
		if n == to {
			return true
		}
	}
	return false
}

func ValidNextStatuses(from string) []string {
	return allowedTransitions[from]
}

// FormatOrderNo formats GMC-YYMMDD-NNNN in Asia/Jakarta date.
func FormatOrderNo(now time.Time, loc *time.Location, seq int32) string {
	t := now.In(loc)
	datePart := t.Format("060102")
	if seq > 9999 {
		return fmt.Sprintf("GMC-%s-%d", datePart, seq)
	}
	return fmt.Sprintf("GMC-%s-%04d", datePart, seq)
}

// RoundPrice adjusts and rounds a price according to kind ("percent", "fixed", "set") and roundingUnit (1, 100, 1000).
func AdjustAndRoundPrice(current int64, kind string, val int64, roundingUnit int64) int64 {
	var next int64
	switch strings.ToLower(kind) {
	case "percent":
		next = current + (current*val)/100
	case "fixed":
		next = current + val
	case "set":
		next = val
	default:
		next = current
	}
	if next < 0 {
		next = 0
	}
	if roundingUnit > 1 {
		half := roundingUnit / 2
		next = ((next + half) / roundingUnit) * roundingUnit
	}
	return next
}
