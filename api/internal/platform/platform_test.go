package platform

import (
	"strings"
	"testing"
)

func TestNormalizePhone(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"0822-3255-4674", "6282232554674", false},
		{"+62 812 3456 7890", "6281234567890", false},
		{"628123456789", "628123456789", false},
		{"8123456789", "628123456789", false},
		{"123", "", true},
	}
	for _, tc := range cases {
		got, err := NormalizePhone(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("NormalizePhone(%q) expected error, got %q", tc.in, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("NormalizePhone(%q) = (%q, %v), want (%q, nil)", tc.in, got, err, tc.want)
		}
	}
}

func TestFormatIDRNumber(t *testing.T) {
	if got := FormatIDRNumber(1234567); got != "1.234.567" {
		t.Errorf("FormatIDRNumber(1234567) = %q, want 1.234.567", got)
	}
	if got := FormatIDRNumber(0); got != "0" {
		t.Errorf("FormatIDRNumber(0) = %q, want 0", got)
	}
}

func TestPasswordHashAndVerify(t *testing.T) {
	hash, err := HashPassword("ChangeMe-Admin-123!")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if !VerifyPassword("ChangeMe-Admin-123!", hash) {
		t.Errorf("VerifyPassword returned false for valid password")
	}
	if VerifyPassword("WrongPassword123!", hash) {
		t.Errorf("VerifyPassword returned true for wrong password")
	}
}

func TestAESGCMEncryptDecrypt(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	ct, err := EncryptAESGCM(key, []byte("JBSWY3DPEHPK3PXP"))
	if err != nil {
		t.Fatalf("EncryptAESGCM failed: %v", err)
	}
	pt, err := DecryptAESGCM(key, ct)
	if err != nil {
		t.Fatalf("DecryptAESGCM failed: %v", err)
	}
	if string(pt) != "JBSWY3DPEHPK3PXP" {
		t.Errorf("decrypted = %q, want JBSWY3DPEHPK3PXP", string(pt))
	}
}

func TestBuildOrderWhatsAppURL(t *testing.T) {
	u := BuildOrderWhatsAppURL(WhatsAppOrderParams{
		WhatsAppNumber: "0822-3255-4674",
		StoreName:      "Global Mandiri Computer",
		OrderNo:        "GMC-260416-0001",
		CustomerName:   "Budi Santoso",
		DeliveryMethod: "pickup",
		TotalIDR:       2450000,
		Items: []WhatsAppOrderLine{
			{Name: "SSD Samsung 990 PRO", VariantLabel: "1TB", Quantity: 1, LineTotalIDR: 2450000},
		},
	})
	if !strings.HasPrefix(u, "https://wa.me/6282232554674?text=") {
		t.Errorf("unexpected WhatsApp URL prefix: %s", u)
	}
	if !strings.Contains(u, "GMC-260416-0001") {
		t.Errorf("expected order number in WhatsApp URL: %s", u)
	}
}

func TestDetectImageMagicBytes(t *testing.T) {
	jpegBytes := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 0, 0, 0, 0, 0, 0, 0}
	if got := DetectImageMagicBytes(jpegBytes); got != "image/jpeg" {
		t.Errorf("DetectImageMagicBytes(JPEG) = %q, want image/jpeg", got)
	}
	svgBytes := []byte("<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>")
	if got := DetectImageMagicBytes(svgBytes); got != "" {
		t.Errorf("DetectImageMagicBytes(SVG) = %q, want empty (rejected)", got)
	}
}
