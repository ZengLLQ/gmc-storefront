package orders

import (
	"testing"
	"time"
)

func TestOrderStateMachine(t *testing.T) {
	if !CanTransition("menunggu_konfirmasi", "diproses") {
		t.Errorf("expected menunggu_konfirmasi -> diproses to be valid")
	}
	if !CanTransition("diproses", "siap_diambil") {
		t.Errorf("expected diproses -> siap_diambil to be valid")
	}
	if !CanTransition("diproses", "dikirim") {
		t.Errorf("expected diproses -> dikirim to be valid")
	}
	if CanTransition("selesai", "dibatalkan") {
		t.Errorf("expected selesai -> dibatalkan to be rejected")
	}
	if CanTransition("dibatalkan", "diproses") {
		t.Errorf("expected dibatalkan -> diproses to be rejected")
	}
}

func TestFormatOrderNo(t *testing.T) {
	tz, _ := time.LoadLocation("Asia/Jakarta")
	ts := time.Date(2026, 4, 16, 10, 0, 0, 0, tz)
	got := FormatOrderNo(ts, tz, 42)
	if got != "GMC-260416-0042" {
		t.Errorf("FormatOrderNo = %q, want GMC-260416-0042", got)
	}
}

func TestAdjustAndRoundPrice(t *testing.T) {
	got := AdjustAndRoundPrice(1450000, "percent", 5, 1000)
	if got != 1523000 {
		t.Errorf("AdjustAndRoundPrice = %d, want 1523000", got)
	}
}
