package platform

import (
	"fmt"
	"net/url"
	"strings"
)

type WhatsAppOrderLine struct {
	Name         string
	VariantLabel string
	Quantity     int
	LineTotalIDR int64
}

type WhatsAppOrderParams struct {
	WhatsAppNumber string
	StoreName      string
	OrderNo        string
	CustomerName   string
	DeliveryMethod string
	City           string
	Items          []WhatsAppOrderLine
	TotalIDR       int64
}

// BuildOrderWhatsAppURL builds the exact Indonesian WhatsApp confirmation link specified in Section 9.
func BuildOrderWhatsAppURL(p WhatsAppOrderParams) string {
	methodStr := "Ambil di toko"
	if p.DeliveryMethod == "delivery" {
		if strings.TrimSpace(p.City) != "" {
			methodStr = "Kirim ke " + strings.TrimSpace(p.City)
		} else {
			methodStr = "Kirim"
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Halo %s, saya ingin konfirmasi pesanan:\n\n", p.StoreName)
	fmt.Fprintf(&b, "No. Pesanan: %s\n", p.OrderNo)
	fmt.Fprintf(&b, "Nama: %s\n", p.CustomerName)
	fmt.Fprintf(&b, "Metode: %s\n\n", methodStr)
	b.WriteString("Pesanan:\n")

	limit := len(p.Items)
	extra := 0
	if limit > 15 {
		extra = limit - 15
		limit = 15
	}

	for i := 0; i < limit; i++ {
		it := p.Items[i]
		label := strings.TrimSpace(it.VariantLabel)
		if label != "" {
			fmt.Fprintf(&b, "%d. %s (%s) x%d - Rp %s\n", i+1, it.Name, label, it.Quantity, FormatIDRNumber(it.LineTotalIDR))
		} else {
			fmt.Fprintf(&b, "%d. %s x%d - Rp %s\n", i+1, it.Name, it.Quantity, FormatIDRNumber(it.LineTotalIDR))
		}
	}
	if extra > 0 {
		fmt.Fprintf(&b, "+%d produk lainnya\n", extra)
	}

	fmt.Fprintf(&b, "Total: Rp %s\n\n", FormatIDRNumber(p.TotalIDR))
	b.WriteString("Mohon info total ongkir & ketersediaan. Terima kasih.")

	num := NormalizePhoneOrKeep(p.WhatsAppNumber)
	return fmt.Sprintf("https://wa.me/%s?text=%s", num, url.QueryEscape(b.String()))
}

func BuildProductWhatsAppURL(waNumber, productName, productURL string) string {
	msg := fmt.Sprintf("Halo, saya tertarik dengan %s (%s)", productName, productURL)
	num := NormalizePhoneOrKeep(waNumber)
	return fmt.Sprintf("https://wa.me/%s?text=%s", num, url.QueryEscape(msg))
}

func BuildContactWhatsAppURL(waNumber, storeName, senderName, subject, message string) string {
	msg := fmt.Sprintf("Halo %s, saya %s ingin bertanya perihal [%s]:\n%s", storeName, senderName, subject, message)
	num := NormalizePhoneOrKeep(waNumber)
	return fmt.Sprintf("https://wa.me/%s?text=%s", num, url.QueryEscape(msg))
}

func RenderAdminOrderWhatsApp(customerPhone, templateText string, vars map[string]string) (string, string) {
	rendered := templateText
	for k, v := range vars {
		rendered = strings.ReplaceAll(rendered, "{{"+k+"}}", v)
	}
	num := NormalizePhoneOrKeep(customerPhone)
	waURL := fmt.Sprintf("https://wa.me/%s?text=%s", num, url.QueryEscape(rendered))
	return waURL, rendered
}
