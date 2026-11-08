package platform

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

type Clock interface {
	Now() time.Time
}

type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now().UTC() }

type IDGenerator interface {
	NewID() uuid.UUID
}

type UUIDv7Generator struct{}

func (UUIDv7Generator) NewID() uuid.UUID {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.New()
	}
	return id
}

func NewUUID() uuid.UUID {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.New()
	}
	return id
}

var nonDigitRe = regexp.MustCompile(`[^0-9+]`)

// NormalizePhone validates Indonesian phone formats (08xx..., +628xx..., 628xx...)
// and normalizes them to 628xxxxxxxxxx (10-15 digits total).
func NormalizePhone(raw string) (string, error) {
	cleaned := nonDigitRe.ReplaceAllString(strings.TrimSpace(raw), "")
	cleaned = strings.TrimPrefix(cleaned, "+")
	if strings.HasPrefix(cleaned, "08") {
		cleaned = "62" + cleaned[1:]
	} else if strings.HasPrefix(cleaned, "8") {
		cleaned = "62" + cleaned
	}
	if !strings.HasPrefix(cleaned, "628") {
		return "", fmt.Errorf("Nomor WhatsApp harus diawali 08 atau 628")
	}
	if len(cleaned) < 10 || len(cleaned) > 15 {
		return "", fmt.Errorf("Nomor WhatsApp harus terdiri dari 10 hingga 14 digit")
	}
	for _, r := range cleaned {
		if r < '0' || r > '9' {
			return "", fmt.Errorf("Nomor WhatsApp hanya boleh berisi angka")
		}
	}
	return cleaned, nil
}

func NormalizePhoneOrKeep(raw string) string {
	if p, err := NormalizePhone(raw); err == nil {
		return p
	}
	return strings.TrimPrefix(strings.TrimSpace(raw), "+")
}

// MaskPhone masks all but the last 3 digits of a phone number for safe logging.
func MaskPhone(phone string) string {
	if len(phone) <= 3 {
		return "***"
	}
	return strings.Repeat("*", len(phone)-3) + phone[len(phone)-3:]
}

// FormatIDRNumber formats an integer rupiah amount with dot thousand separators (e.g. 1.234.567).
func FormatIDRNumber(amount int64) string {
	neg := amount < 0
	if neg {
		amount = -amount
	}
	s := strconv.FormatInt(amount, 10)
	var out []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, byte(c))
	}
	if neg {
		return "-" + string(out)
	}
	return string(out)
}

func HasControlChars(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return true
		}
	}
	return false
}

// EscapeCSVCell prevents CSV formula injection by prefixing cells starting with =, +, -, @, \t, \r with a single quote.
func EscapeCSVCell(val string) string {
	if val == "" {
		return ""
	}
	first := val[0]
	if first == '=' || first == '+' || first == '-' || first == '@' || first == '\t' || first == '\r' {
		return "'" + val
	}
	return val
}
