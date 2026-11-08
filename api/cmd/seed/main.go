package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"gmc/api/internal/platform"
)

type csvVariantGroup struct {
	Option  string             `json:"option"`
	Display string             `json:"display"`
	Values  []csvVariantOption `json:"values"`
}

type csvVariantOption struct {
	Label    string `json:"label"`
	Swatch   string `json:"swatch,omitempty"`
	Sku      string `json:"sku"`
	PriceIdr int64  `json:"price_idr"`
	CostIdr  int64  `json:"cost_idr"`
	Stock    int    `json:"stock"`
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := platform.LoadConfig()
	if err != nil {
		logger.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect to db", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	logger.Info("seeding Global Mandiri Computer database...")

	// 1. Seed Users (Admin, Staff, Demo Customers)
	adminID := seedUsers(ctx, pool, cfg, logger)

	// 2. Seed Site Settings, Announcements, Promo Slides, FAQs, Vouchers
	seedCMSAndSettings(ctx, pool, adminID, logger)

	// 3. Seed Categories & Brands
	catMap := seedCategories(ctx, pool, logger)
	brandMap := seedBrands(ctx, pool, logger)

	// 4. Seed Products from CSV
	seedProductsFromCSV(ctx, pool, catMap, brandMap, adminID, logger)

	// 5. Seed Historical Orders & Contact Messages & Subscribers
	seedDemoOrdersAndInbox(ctx, pool, adminID, logger)

	logger.Info("database seeding completed successfully")
}

func deterministicUUID(namespace, key string) uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("gmc:"+namespace+":"+key))
}

func makeEditorialHardwareSVG(brand, name, categorySlug string, isSecondary bool) string {
	bg := "#F2EFE8"
	accent := "#1A1917"
	sub := "#78746C"
	badge := strings.ToUpper(brand)
	shortName := name
	if len(shortName) > 38 {
		shortName = shortName[:38] + "..."
	}

	var graphic string
	if !isSecondary {
		// Primary clean technical architectural silhouette
		switch categorySlug {
		case "laptop":
			graphic = `<rect x="210" y="220" width="380" height="240" rx="8" fill="#1F1E1C" stroke="#111111" stroke-width="3"/><rect x="226" y="236" width="348" height="206" fill="#2C2A27"/><path d="M165 460 L635 460 L605 482 L195 482 Z" fill="#C8C3B8" stroke="#111111" stroke-width="2"/>`
		case "monitor", "proyektor":
			graphic = `<rect x="175" y="195" width="450" height="270" rx="4" fill="#191816" stroke="#111111" stroke-width="3"/><rect x="191" y="211" width="418" height="238" fill="#2A2825"/><rect x="378" y="465" width="44" height="58" fill="#3A3834"/><rect x="305" y="520" width="190" height="12" rx="2" fill="#191816"/>`
		case "gpu":
			graphic = `<rect x="175" y="260" width="450" height="190" rx="6" fill="#21201E" stroke="#111111" stroke-width="3"/><circle cx="295" cy="355" r="66" fill="#151413" stroke="#5F5C55" stroke-width="2"/><circle cx="505" cy="355" r="66" fill="#151413" stroke="#5F5C55" stroke-width="2"/><circle cx="295" cy="355" r="22" fill="#2E2C29"/><circle cx="505" cy="355" r="22" fill="#2E2C29"/>`
		case "cpu":
			graphic = `<rect x="260" y="220" width="280" height="280" rx="12" fill="#C9C5BC" stroke="#111111" stroke-width="3"/><rect x="295" y="255" width="210" height="210" rx="6" fill="#E3E0D8" stroke="#5F5C55" stroke-width="2"/><rect x="335" y="315" width="130" height="90" fill="#191816"/>`
		case "starlink", "networking":
			graphic = `<rect x="245" y="185" width="310" height="330" rx="10" fill="#EAE6DD" stroke="#111111" stroke-width="3"/><line x1="400" y1="515" x2="400" y2="570" stroke="#111111" stroke-width="10"/><line x1="330" y1="570" x2="470" y2="570" stroke="#111111" stroke-width="8"/>`
		default:
			graphic = `<rect x="235" y="195" width="330" height="330" rx="8" fill="#22211F" stroke="#111111" stroke-width="3"/><rect x="265" y="225" width="270" height="270" rx="4" fill="#2D2B28" stroke="#5F5C55" stroke-width="1.5"/><circle cx="400" cy="360" r="54" fill="none" stroke="#8B877E" stroke-width="2"/>`
		}
	} else {
		bg = "#EAE5DA"
		// Secondary isometric / studio detail specification card view
		graphic = `<rect x="190" y="190" width="420" height="330" rx="6" fill="#171615" stroke="#111111" stroke-width="3"/><line x1="230" y1="265" x2="570" y2="265" stroke="#3D3A35" stroke-width="2"/><line x1="230" y1="335" x2="520" y2="335" stroke="#3D3A35" stroke-width="2"/><line x1="230" y1="405" x2="480" y2="405" stroke="#3D3A35" stroke-width="2"/><circle cx="535" cy="425" r="34" fill="#2B2926" stroke="#8B877E" stroke-width="2"/>`
	}

	viewLabel := "STUDIO VIEW 01"
	if isSecondary {
		viewLabel = "DETAIL SPEC 02"
	}

	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 800 800" width="800" height="800">
		<rect width="800" height="800" fill="%s"/>
		<text x="64" y="76" font-family="Inter, sans-serif" font-size="15" font-weight="500" letter-spacing="3" fill="%s">%s</text>
		<text x="736" y="76" text-anchor="end" font-family="Inter, sans-serif" font-size="13" font-weight="400" letter-spacing="2" fill="%s">%s</text>
		<line x1="64" y1="98" x2="736" y2="98" stroke="#DAD6CC" stroke-width="1"/>
		%s
		<line x1="64" y1="680" x2="736" y2="680" stroke="#DAD6CC" stroke-width="1"/>
		<text x="64" y="720" font-family="Inter, sans-serif" font-size="20" font-weight="500" fill="%s">%s</text>
		<text x="64" y="748" font-family="Inter, sans-serif" font-size="13" font-weight="400" letter-spacing="1.5" fill="%s">GLOBAL MANDIRI COMPUTER · SURABAYA</text>
	</svg>`, bg, accent, htmlEscape(badge), sub, viewLabel, graphic, accent, htmlEscape(shortName), sub)

	return "data:image/svg+xml;utf8," + url.PathEscape(svg)
}

func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")
	return r.Replace(s)
}

func seedUsers(ctx context.Context, pool *pgxpool.Pool, cfg *platform.Config, logger *slog.Logger) uuid.UUID {
	adminID := deterministicUUID("user", cfg.AdminEmail)
	adminHash, _ := platform.HashPassword(cfg.AdminPassword)
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, name, phone, role, is_active, must_change_password)
		VALUES ($1, $2, $3, 'Administrator GMC', '6282232554674', 'admin', TRUE, FALSE)
		ON CONFLICT (email) DO UPDATE SET password_hash = EXCLUDED.password_hash, role = 'admin', is_active = TRUE
	`, adminID, strings.ToLower(cfg.AdminEmail), adminHash)
	if err != nil {
		logger.Error("failed to seed admin user", "err", err)
	}
	_ = pool.QueryRow(ctx, `SELECT id FROM users WHERE email = $1`, strings.ToLower(cfg.AdminEmail)).Scan(&adminID)

	if cfg.AppEnv == "development" {
		staffID := deterministicUUID("user", "staff@gmc.local")
		staffHash, _ := platform.HashPassword("ChangeMe-Staff-123!")
		_, _ = pool.Exec(ctx, `
			INSERT INTO users (id, email, password_hash, name, phone, role, is_active, must_change_password)
			VALUES ($1, 'staff@gmc.local', $2, 'Staf Operasional GMC', '6281234567890', 'staff', TRUE, FALSE)
			ON CONFLICT (email) DO UPDATE SET password_hash = EXCLUDED.password_hash, role = 'staff', is_active = TRUE
		`, staffID, staffHash)
	}

	custHash, _ := platform.HashPassword("Customer-123!")
	customers := []struct {
		Email string
		Name  string
		Phone string
		City  string
		Prov  string
		Addr  string
	}{
		{"budi.santoso@example.com", "Budi Santoso", "6281231112233", "Surabaya", "Jawa Timur", "Jl. Raya Darmo No. 88, Tegalsari"},
		{"siti.rahma@example.com", "Siti Rahma", "6281332223344", "Sidoarjo", "Jawa Timur", "Perum Pondok Jati Blok C-12"},
		{"andika.pratama@example.com", "Andika Pratama", "6281134445566", "Malang", "Jawa Timur", "Jl. Soekarno Hatta No. 45, Lowokwaru"},
		{"kevin.wijaya@example.com", "Kevin Wijaya", "6281835556677", "Surabaya", "Jawa Timur", "Jl. Mayjen Sungkono Kav. 14"},
		{"nadia.kusuma@example.com", "Nadia Kusuma", "6281936667788", "Gresik", "Jawa Timur", "Jl. Veteran No. 19, Kebomas"},
	}

	for _, c := range customers {
		uID := deterministicUUID("user", c.Email)
		_, _ = pool.Exec(ctx, `
			INSERT INTO users (id, email, password_hash, name, phone, role, is_active)
			VALUES ($1, $2, $3, $4, $5, 'customer', TRUE)
			ON CONFLICT (email) DO NOTHING
		`, uID, c.Email, custHash, c.Name, c.Phone)

		addrID := deterministicUUID("addr", c.Email)
		_, _ = pool.Exec(ctx, `
			INSERT INTO addresses (id, user_id, label, recipient_name, phone, street, city, province, postal_code, is_default)
			VALUES ($1, $2, 'Rumah', $3, $4, $5, $6, $7, '60264', TRUE)
			ON CONFLICT (id) DO NOTHING
		`, addrID, uID, c.Name, c.Phone, c.Addr, c.City, c.Prov)
	}

	return adminID
}

func seedCMSAndSettings(ctx context.Context, pool *pgxpool.Pool, adminID uuid.UUID, logger *slog.Logger) {
	settingsGroups := map[string]any{
		"store": map[string]any{
			"name":     "Global Mandiri Computer",
			"tagline":  "Rakit. Upgrade. Terhubung.",
			"address":  "ITC Mega Grosir Lt. 2 Blok B/7, Surabaya",
			"whatsapp": "6282232554674",
			"phone":    "0822-3255-4674",
			"email":    "veneratesman@gmail.com",
			"mapsUrl":  "https://www.google.com/maps/search/?api=1&query=ITC+Mega+Grosir+Surabaya",
			"hours":    "Senin – Sabtu, 09.00 – 17.00 WIB",
			"social": map[string]string{
				"whatsapp": "https://wa.me/6282232554674",
			},
		},
		"operational_status": map[string]any{
			"isOpen":   true,
			"message":  "Toko buka dan melayani pesanan di ITC Mega Grosir Lt. 2 Blok B/7, Surabaya.",
			"reopenAt": nil,
		},
		"announcement": map[string]any{
			"enabled":        true,
			"rotateSeconds":  5,
		},
		"payment": map[string]any{
			"methods": []string{"transfer", "cod_pickup", "whatsapp"},
			"bankAccounts": []map[string]string{
				{"bank": "BCA", "number": "088-912-3456", "holder": "Global Mandiri Computer"},
				{"bank": "Mandiri", "number": "142-00-9876543-2", "holder": "Global Mandiri Computer"},
			},
		},
		"shipping": map[string]any{
			"pickupAddress": "ITC Mega Grosir Lt. 2 Blok B/7, Surabaya",
			"couriers":      []string{"JNE", "J&T", "SiCepat", "AnterAja", "Pos Indonesia", "Kurir Instan Surabaya"},
		},
		"checkout": map[string]any{
			"allowGuest":          true,
			"maxQtyPerLine":       20,
			"autoCancelHours":     48,
		},
		"whatsapp_templates": map[string]any{
			"confirm_order":   "Halo {customerName}, terima kasih telah memesan di Global Mandiri Computer.\nPesanan *{orderNo}* telah kami terima dan stok sedang kami siapkan.",
			"payment_request": "Halo {customerName}, pesanan *{orderNo}* senilai *{totalFormatted}* siap dilanjutkan ke pembayaran.",
			"shipped":         "Halo {customerName}, pesanan *{orderNo}* telah dikirim melalui *{courier}* dengan nomor resi *{trackingNo}*.",
			"ready_pickup":    "Halo {customerName}, pesanan *{orderNo}* sudah siap diambil di ITC Mega Grosir Lt. 2 Blok B/7, Surabaya.",
		},
		"seo": map[string]any{
			"defaultTitle":       "Global Mandiri Computer (GMC) — Komputer, Komponen PC, Laptop & Starlink Surabaya",
			"defaultDescription": "Toko komputer, komponen rakitan PC, laptop, monitor, proyektor, printer, networking, dan Starlink di ITC Mega Grosir Lt. 2 Blok B/7, Surabaya.",
		},
	}

	for key, val := range settingsGroups {
		b, _ := json.Marshal(val)
		_, _ = pool.Exec(ctx, `
			INSERT INTO site_settings (key, value, version, updated_by, updated_at)
			VALUES ($1, $2, 1, $3, now())
			ON CONFLICT (key) DO NOTHING
		`, key, b, adminID)
	}

	announcements := []struct {
		Text string
		Link string
		Ord  int
	}{
		{"Ambil langsung di ITC Mega Grosir Lt. 2 Blok B/7", "/contact", 1},
		{"Pesan mudah via WhatsApp", "/shop", 2},
		{"Garansi sesuai ketentuan resmi brand", "/contact#garansi", 3},
	}
	for _, a := range announcements {
		id := deterministicUUID("announcement", a.Text)
		_, _ = pool.Exec(ctx, `
			INSERT INTO announcements (id, text, link_url, sort_order, is_active)
			VALUES ($1, $2, $3, $4, TRUE)
			ON CONFLICT (id) DO NOTHING
		`, id, a.Text, a.Link, a.Ord)
	}

	slides := []struct {
		Label    string
		Headline string
		Body     string
		CtaLabel string
		CtaUrl   string
		Img      string
		Ord      int
	}{
		{
			Label:    "KOMPUTER & KOMPONEN PILIHAN",
			Headline: "PC Gaming & Rakitan Presisi",
			Body:     "Dirakit dengan manajemen kabel bersih, komponen bergaransi resmi distributor Indonesia, dan diuji stabilitas termal di workshop kami di ITC Mega Grosir Surabaya.",
			CtaLabel: "Jelajahi Komponen",
			CtaUrl:   "/shop?category=komputer-rakitan",
			Img:      makeEditorialHardwareSVG("GMC CUSTOM BUILD", "PC Gaming & Workstation Rakitan", "komputer-rakitan", false),
			Ord:      1,
		},
		{
			Label:    "KONEKTIVITAS TANPA BATAS",
			Headline: "Networking & Starlink Siap Kirim",
			Body:     "Solusi internet satelit Starlink Standard V4, Starlink Mini, serta infrastruktur jaringan Ubiquiti UniFi, MikroTik, dan TP-Link untuk rumah maupun lokasi operasional terpencil.",
			CtaLabel: "Lihat Koleksi Starlink",
			CtaUrl:   "/shop?category=starlink",
			Img:      makeEditorialHardwareSVG("STARLINK & UNIFI", "Konektivitas Satelit & Enterprise", "starlink", false),
			Ord:      2,
		},
	}
	for _, sl := range slides {
		id := deterministicUUID("slide", sl.Headline)
		_, _ = pool.Exec(ctx, `
			INSERT INTO promo_slides (id, label, headline, body, cta_label, cta_url, image_url, image_mobile_url, sort_order, is_active)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $7, $8, TRUE)
			ON CONFLICT (id) DO NOTHING
		`, id, sl.Label, sl.Headline, sl.Body, sl.CtaLabel, sl.CtaUrl, sl.Img, sl.Ord)
	}

	faqs := []struct {
		Anchor   string
		Question string
		Answer   string
		Ord      int
	}{
		{"pengiriman", "Bagaimana pengiriman pesanan dan estimasi ongkos kirim?", "Anda dapat memilih Ambil di Toko (ITC Mega Grosir Lt. 2 Blok B/7, Surabaya — gratis biaya ambil) atau Kirim ke Alamat menggunakan ekspedisi JNE, J&T, SiCepat, AnterAja, maupun kurir instan Surabaya. Ongkos kirim dan asuransi pengiriman dihitung secara transparan dan dikonfirmasi oleh admin melalui WhatsApp sebelum Anda melakukan pembayaran.", 1},
		{"pembayaran", "Apa saja metode pembayaran yang tersedia?", "Kami melayani pembayaran melalui Transfer Bank resmi (BCA & Mandiri), Bayar saat Ambil di Toko (ITC Mega Grosir Lt. 2 Blok B/7, Surabaya), serta konfirmasi pesanan langsung via WhatsApp bersama admin GMC.", 2},
		{"garansi", "Bagaimana ketentuan garansi produk di Global Mandiri Computer?", "Seluruh produk baru yang kami jual mengikuti ketentuan garansi resmi masing-masing brand dan distributor resmi di Indonesia. Simpan nota pembelian GMC dan kemasan produk (lengkap dengan stiker nomor seri distributor) untuk memudahkan bantuan klaim garansi.", 3},
		{"pengembalian", "Apakah produk yang sudah dibeli dapat ditukar jika terdapat kendala pabrik?", "Apabila unit mengalami kendala fungsi pabrik saat diterima, silakan hubungi admin WhatsApp kami maksimal 2x24 jam sejak barang diterima beserta video unboxing utuh tanpa jeda untuk proses pemeriksaan dan tindak lanjut sesuai ketentuan distributor.", 4},
		{"rakit-pc", "Apakah GMC menerima konsultasi spesifikasi dan jasa rakit PC?", "Tentu. Anda dapat berkonsultasi langsung di toko kami di ITC Mega Grosir Lt. 2 Blok B/7 maupun melalui WhatsApp 0822-3255-4674. Tim kami membantu memastikan kompatibilitas soket CPU, RAM, dimensi casing, hingga kapasitas daya PSU.", 5},
		{"starlink", "Apakah perangkat Starlink yang dijual baru dan dapat diaktivasi di seluruh Indonesia?", "Ya, unit Starlink Standard V4 dan Starlink Mini yang kami sediakan adalah perangkat baru tersegel dan dapat diaktivasi secara mandiri menggunakan akun resmi Starlink di Indonesia.", 6},
	}
	for _, f := range faqs {
		id := deterministicUUID("faq", f.Anchor)
		_, _ = pool.Exec(ctx, `
			INSERT INTO faqs (id, anchor, question, answer, sort_order, is_active)
			VALUES ($1, $2, $3, $4, $5, TRUE)
			ON CONFLICT (id) DO NOTHING
		`, id, f.Anchor, f.Question, f.Answer, f.Ord)
	}

	// Seed 3 Vouchers
	now := time.Now()
	expiredEnd := now.Add(-24 * time.Hour)
	expiredStart := now.Add(-30 * 24 * time.Hour)
	_, _ = pool.Exec(ctx, `
		INSERT INTO vouchers (id, code, description, kind, value, min_subtotal_idr, max_discount_idr, per_user_limit, is_active)
		VALUES
		  ($1, 'HEMAT100K', 'Potongan Rp 100.000 untuk belanja minimal Rp 2.000.000', 'fixed', 100000, 2000000, NULL, 3, TRUE),
		  ($2, 'RAKITPC5', 'Diskon 5% hingga Rp 500.000 untuk belanja minimal Rp 5.000.000', 'percent', 5, 5000000, 500000, 2, TRUE)
		ON CONFLICT (code) DO NOTHING
	`, deterministicUUID("voucher", "HEMAT100K"), deterministicUUID("voucher", "RAKITPC5"))

	_, _ = pool.Exec(ctx, `
		INSERT INTO vouchers (id, code, description, kind, value, min_subtotal_idr, starts_at, ends_at, per_user_limit, is_active)
		VALUES ($1, 'EXPIRED10', 'Voucher promo bulan lalu (telah berakhir)', 'percent', 10, 1000000, $2, $3, 1, TRUE)
		ON CONFLICT (code) DO NOTHING
	`, deterministicUUID("voucher", "EXPIRED10"), expiredStart, expiredEnd)

	logger.Info("seeded CMS settings, announcements, promo slides, FAQs, and vouchers")
}

func seedCategories(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger) map[string]uuid.UUID {
	categories := []struct {
		Slug string
		Name string
		Desc string
		Ord  int
	}{
		{"komputer-rakitan", "PC & Komponen Rakitan", "Desktop rakitan siap pakai untuk kerja, studio kreatif, dan gaming performa tinggi.", 1},
		{"laptop", "Laptop", "Ultrabook bisnis, laptop kreator OLED, dan laptop gaming bergaransi resmi.", 2},
		{"monitor", "Monitor", "Monitor kalibrasi warna 4K, layar kerja ergonomis, dan monitor gaming refresh rate tinggi.", 3},
		{"proyektor", "Proyektor", "Proyektor ruang rapat 3LCD dan smart projector 4K untuk ruang keluarga.", 4},
		{"cpu", "Prosesor (CPU)", "Prosesor desktop Intel Core dan AMD Ryzen generasi terbaru.", 5},
		{"gpu", "Kartu Grafis (VGA / GPU)", "Kartu grafis untuk gaming, rendering 3D, dan komputasi AI.", 6},
		{"motherboard", "Motherboard", "Mainboard ATX, Micro-ATX, dan Mini-ITX bersoket AM5 dan LGA1700.", 7},
		{"ram", "Memori (RAM)", "Modul RAM desktop DDR5, DDR4, dan SODIMM laptop.", 8},
		{"ssd", "SSD Internal & Portable", "Penyimpanan cepat NVMe PCIe Gen4 dan SSD eksternal tahan banting.", 9},
		{"hdd", "Hard Disk (HDD & NAS)", "Hard disk internal PC, hard disk NAS 24/7 CMR, dan HDD portabel.", 10},
		{"psu", "Power Supply (PSU)", "Unit catu daya bersertifikasi 80 Plus Gold ATX 3.0 / 3.1 modular.", 11},
		{"casing", "Casing PC", "Casing airflow, dual-chamber kaca tempered, dan sasis kayu minimalis.", 12},
		{"cooler-air", "CPU Air Cooler", "Pendingin udara menara tunggal dan ganda yang hening dan bertenaga.", 13},
		{"cooler-aio", "Liquid Cooler (AIO)", "Pendingin cair All-in-One 240mm dan 360mm.", 14},
		{"fan-casing", "Fan Casing", "Kipas PWM presisi untuk sirkulasi udara casing dan radiator.", 15},
		{"keyboard", "Keyboard", "Keyboard mekanikal nirkabel, hot-swappable, dan periferal kerja.", 16},
		{"mouse", "Mouse", "Mouse produktivitas ergonomis dan mouse esports ultra-ringan.", 17},
		{"speaker", "Speaker & Audio", "Speaker bookshelf aktif, studio monitor meja, dan headset.", 18},
		{"ups", "UPS & Stabilizer", "Uninterruptible Power Supply untuk perlindungan daya komputer dan server.", 19},
		{"starlink", "Starlink", "Perangkat internet satelit SpaceX Starlink Standard V4, Mini, dan aksesoris mounting.", 20},
		{"networking", "Networking & Aksesoris", "Router Wi-Fi 6/7, Mesh, Access Point UniFi, MikroTik, switch PoE, dan charger GaN.", 21},
		{"printer", "Printer & Scanner", "Printer ink tank infus pabrik dan printer laser monokrom kantor.", 22},
	}

	catMap := make(map[string]uuid.UUID, len(categories))
	for _, c := range categories {
		id := deterministicUUID("category", c.Slug)
		img := makeEditorialHardwareSVG("KOLEKSI GMC", c.Name, c.Slug, false)
		_, err := pool.Exec(ctx, `
			INSERT INTO categories (id, slug, name, description, image_url, sort_order, is_active)
			VALUES ($1, $2, $3, $4, $5, $6, TRUE)
			ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name, description = EXCLUDED.description, image_url = EXCLUDED.image_url, sort_order = EXCLUDED.sort_order
		`, id, c.Slug, c.Name, c.Desc, img, c.Ord)
		if err != nil {
			logger.Error("failed to seed category", "slug", c.Slug, "err", err)
		}
		_ = pool.QueryRow(ctx, `SELECT id FROM categories WHERE slug = $1`, c.Slug).Scan(&id)
		catMap[c.Slug] = id
	}
	return catMap
}

func seedBrands(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger) map[string]uuid.UUID {
	brands := []struct {
		Slug string
		Name string
	}{
		{"gigabyte", "GIGABYTE"},
		{"msi", "MSI"},
		{"dell", "Dell"},
		{"apple", "Apple"},
		{"acer", "Acer"},
		{"asus", "ASUS"},
		{"asrock", "ASRock"},
		{"g-skill", "G.Skill"},
		{"corsair", "Corsair"},
		{"samsung", "Samsung"},
		{"seagate", "Seagate"},
		{"wd", "WD"},
		{"lenovo", "Lenovo"},
		{"razer", "Razer"},
		{"logitech", "Logitech"},
		{"edifier", "Edifier"},
		{"sk-hynix", "SK hynix"},
		{"ugreen", "UGREEN"},
		{"anker", "Anker"},
		{"vention", "Vention"},
		{"tp-link", "TP-Link"},
		{"ubiquiti", "Ubiquiti"},
		{"d-link", "D-Link"},
		{"zotac", "Zotac"},
		{"intel", "Intel"},
		{"amd", "AMD"},
		{"apc", "APC"},
		{"fractal-design", "Fractal Design"},
		{"thermaltake", "Thermaltake"},
		{"lian-li", "Lian Li"},
		{"be-quiet", "be quiet!"},
		{"fsp", "FSP"},
		{"brother", "Brother"},
		{"epson", "Epson"},
		{"canon", "Canon"},
		{"nvidia", "NVIDIA"},
		{"kingston", "Kingston"},
		{"crucial", "Crucial"},
		{"sandisk", "SanDisk"},
		{"toshiba", "Toshiba"},
		{"cooler-master", "Cooler Master"},
		{"nzxt", "NZXT"},
		{"noctua", "Noctua"},
		{"deepcool", "DeepCool"},
		{"seasonic", "Seasonic"},
		{"hp", "HP"},
		{"lg", "LG"},
		{"benq", "BenQ"},
		{"viewsonic", "ViewSonic"},
		{"aoc", "AOC"},
		{"philips", "Philips"},
		{"hyperx", "HyperX"},
		{"steelseries", "SteelSeries"},
		{"jbl", "JBL"},
		{"creative", "Creative"},
		{"rexus", "Rexus"},
		{"fantech", "Fantech"},
		{"mercusys", "Mercusys"},
		{"tenda", "Tenda"},
		{"mikrotik", "MikroTik"},
		{"netgear", "NETGEAR"},
		{"xgimi", "XGIMI"},
		{"optoma", "Optoma"},
		{"eaton", "Eaton"},
		{"prolink", "Prolink"},
		{"spacex-starlink", "SpaceX (Starlink)"},
	}

	brandMap := make(map[string]uuid.UUID, len(brands))
	for _, b := range brands {
		id := deterministicUUID("brand", b.Slug)
		_, err := pool.Exec(ctx, `
			INSERT INTO brands (id, slug, name, logo_text, is_active)
			VALUES ($1, $2, $3, $3, TRUE)
			ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name, logo_text = EXCLUDED.logo_text
		`, id, b.Slug, b.Name)
		if err != nil {
			logger.Error("failed to seed brand", "slug", b.Slug, "err", err)
		}
		_ = pool.QueryRow(ctx, `SELECT id FROM brands WHERE slug = $1`, b.Slug).Scan(&id)
		brandMap[b.Slug] = id
	}
	return brandMap
}

func seedProductsFromCSV(ctx context.Context, pool *pgxpool.Pool, catMap, brandMap map[string]uuid.UUID, adminID uuid.UUID, logger *slog.Logger) {
	csvPaths := []string{
		filepath.Join("db", "seed", "products.csv"),
		filepath.Join("api", "db", "seed", "products.csv"),
		"/seed/products.csv",
	}
	var f *os.File
	var err error
	for _, p := range csvPaths {
		f, err = os.Open(p)
		if err == nil {
			break
		}
	}
	if f == nil {
		logger.Error("could not find products.csv in any expected location")
		return
	}
	defer f.Close()

	reader := csv.NewReader(f)
	headers, err := reader.Read()
	if err != nil {
		logger.Error("failed to read CSV headers", "err", err)
		return
	}
	colIdx := map[string]int{}
	for i, h := range headers {
		colIdx[strings.TrimSpace(h)] = i
	}

	count := 0
	for {
		rec, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		slug := strings.TrimSpace(rec[colIdx["slug"]])
		name := strings.TrimSpace(rec[colIdx["name"]])
		brandSlug := strings.TrimSpace(rec[colIdx["brand_slug"]])
		catSlug := strings.TrimSpace(rec[colIdx["category_slug"]])
		priceIdr, _ := strconv.ParseInt(strings.TrimSpace(rec[colIdx["price_idr"]]), 10, 64)
		compareStr := strings.TrimSpace(rec[colIdx["compare_at_price_idr"]])
		var comparePtr *int64
		if compareStr != "" {
			if c, err := strconv.ParseInt(compareStr, 10, 64); err == nil && c > 0 {
				comparePtr = &c
			}
		}
		costIdr, _ := strconv.ParseInt(strings.TrimSpace(rec[colIdx["cost_idr"]]), 10, 64)
		sku := strings.TrimSpace(rec[colIdx["sku"]])
		stock, _ := strconv.Atoi(strings.TrimSpace(rec[colIdx["stock"]]))
		lowThresh, _ := strconv.Atoi(strings.TrimSpace(rec[colIdx["low_stock_threshold"]]))
		if lowThresh <= 0 {
			lowThresh = 3
		}
		isFeatured := strings.EqualFold(strings.TrimSpace(rec[colIdx["is_featured"]]), "true")
		isPreorder := strings.EqualFold(strings.TrimSpace(rec[colIdx["is_preorder"]]), "true")
		summary := strings.TrimSpace(rec[colIdx["summary"]])
		highlightsRaw := strings.TrimSpace(rec[colIdx["highlights"]])
		specsRaw := strings.TrimSpace(rec[colIdx["specs_json"]])
		variantsRaw := strings.TrimSpace(rec[colIdx["variants_json"]])

		catID, okCat := catMap[catSlug]
		brandID, okBrand := brandMap[brandSlug]
		if !okCat || !okBrand {
			logger.Warn("skipping product with unknown category or brand", "slug", slug, "cat", catSlug, "brand", brandSlug)
			continue
		}

		highlights := []string{}
		if highlightsRaw != "" {
			for _, h := range strings.Split(highlightsRaw, "|") {
				if strings.TrimSpace(h) != "" {
					highlights = append(highlights, strings.TrimSpace(h))
				}
			}
		}
		if specsRaw == "" {
			specsRaw = "{}"
		}

		description := summary + "\n\n" +
			"Tersedia di Global Mandiri Computer (ITC Mega Grosir Lt. 2 Blok B/7, Surabaya). " +
			"Setiap unit telah melalui pemeriksaan keaslian distribusi resmi dan siap diambil langsung di toko maupun dikirim ke seluruh Indonesia dengan pengemasan aman berlapis."

		prodID := deterministicUUID("product", slug)
		var existingID uuid.UUID
		err = pool.QueryRow(ctx, `SELECT id FROM products WHERE slug = $1`, slug).Scan(&existingID)
		if err == nil {
			continue // Already seeded
		}

		_, err = pool.Exec(ctx, `
			INSERT INTO products (id, slug, name, brand_id, category_id, summary, description, highlights, specs, is_featured, is_active, seo_title, seo_description)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, TRUE, $3, $6)
		`, prodID, slug, name, brandID, catID, summary, description, highlights, []byte(specsRaw), isFeatured)
		if err != nil {
			logger.Error("failed to insert product", "slug", slug, "err", err)
			continue
		}

		// Insert Primary & Secondary studio images
		primaryImg := makeEditorialHardwareSVG(brandSlug, name, catSlug, false)
		secondaryImg := makeEditorialHardwareSVG(brandSlug, name, catSlug, true)
		_, _ = pool.Exec(ctx, `
			INSERT INTO product_images (id, product_id, url, alt, kind, width, height, sort_order)
			VALUES
			  ($1, $2, $3, $4, 'product', 800, 800, 0),
			  ($5, $2, $6, $4, 'detail', 800, 800, 1)
		`, deterministicUUID("img1", slug), prodID, primaryImg, name, deterministicUUID("img2", slug), secondaryImg)

		// Variants & Options
		var groups []csvVariantGroup
		if variantsRaw != "" {
			_ = json.Unmarshal([]byte(variantsRaw), &groups)
		}

		if len(groups) > 0 && len(groups[0].Values) > 0 {
			grp := groups[0]
			optID := deterministicUUID("opt", slug+":"+grp.Option)
			disp := grp.Display
			if disp != "swatch" {
				disp = "pill"
			}
			_, _ = pool.Exec(ctx, `
				INSERT INTO product_options (id, product_id, name, display, sort_order)
				VALUES ($1, $2, $3, $4, 0)
			`, optID, prodID, grp.Option, disp)

			for vIdx, val := range grp.Values {
				valID := deterministicUUID("optval", slug+":"+val.Sku)
				var swatchPtr *string
				if val.Swatch != "" {
					s := val.Swatch
					swatchPtr = &s
				}
				_, _ = pool.Exec(ctx, `
					INSERT INTO product_option_values (id, option_id, label, swatch_hex, sort_order)
					VALUES ($1, $2, $3, $4, $5)
				`, valID, optID, val.Label, swatchPtr, vIdx)

				varID := deterministicUUID("variant", val.Sku)
				_, _ = pool.Exec(ctx, `
					INSERT INTO variants (id, product_id, sku, option_value_ids, price_idr, compare_at_price_idr, cost_idr, stock, low_stock_threshold, is_preorder, is_active)
					VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, TRUE)
				`, varID, prodID, val.Sku, []uuid.UUID{valID}, val.PriceIdr, comparePtr, val.CostIdr, val.Stock, lowThresh, isPreorder)

				if val.Stock > 0 {
					_, _ = pool.Exec(ctx, `
						INSERT INTO stock_movements (id, variant_id, delta, stock_after, reason, actor_id, note)
						VALUES ($1, $2, $3, $3, 'initial', $4, 'Stok awal katalog GMC')
					`, deterministicUUID("sm-init", val.Sku), varID, val.Stock, adminID)
				}
			}
		} else {
			varID := deterministicUUID("variant", sku)
			_, _ = pool.Exec(ctx, `
				INSERT INTO variants (id, product_id, sku, option_value_ids, price_idr, compare_at_price_idr, cost_idr, stock, low_stock_threshold, is_preorder, is_active)
				VALUES ($1, $2, $3, '{}', $4, $5, $6, $7, $8, $9, TRUE)
			`, varID, prodID, sku, priceIdr, comparePtr, costIdr, stock, lowThresh, isPreorder)

			if stock > 0 {
				_, _ = pool.Exec(ctx, `
					INSERT INTO stock_movements (id, variant_id, delta, stock_after, reason, actor_id, note)
					VALUES ($1, $2, $3, $3, 'initial', $4, 'Stok awal katalog GMC')
				`, deterministicUUID("sm-init", sku), varID, stock, adminID)
			}
		}
		count++
	}
	logger.Info("seeded catalog products", "newProducts", count)
}

func seedDemoOrdersAndInbox(ctx context.Context, pool *pgxpool.Pool, adminID uuid.UUID, logger *slog.Logger) {
	var existingOrders int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM orders`).Scan(&existingOrders)
	if existingOrders > 0 {
		return
	}

	type seedVariant struct {
		ID       uuid.UUID
		SKU      string
		Name     string
		Slug     string
		Brand    string
		PriceIdr int64
		CostIdr  int64
		ImgURL   string
	}
	rows, err := pool.Query(ctx, `
		SELECT pv.id, pv.sku, p.name, p.slug, b.name, pv.price_idr, COALESCE(pv.cost_idr, 0),
		       COALESCE((SELECT url FROM product_images WHERE product_id = p.id LIMIT 1), '')
		FROM product_variants pv
		JOIN products p ON p.id = pv.product_id
		JOIN brands b ON b.id = p.brand_id
		WHERE pv.stock >= 3
		ORDER BY pv.price_idr DESC
		LIMIT 25
	`)
	if err != nil {
		return
	}
	var variants []seedVariant
	for rows.Next() {
		var sv seedVariant
		if rows.Scan(&sv.ID, &sv.SKU, &sv.Name, &sv.Slug, &sv.Brand, &sv.PriceIdr, &sv.CostIdr, &sv.ImgURL) == nil {
			variants = append(variants, sv)
		}
	}
	rows.Close()
	if len(variants) < 5 {
		return
	}

	var customerIDs []uuid.UUID
	uRows, _ := pool.Query(ctx, `SELECT id FROM users WHERE role = 'customer' LIMIT 5`)
	for uRows.Next() {
		var id uuid.UUID
		if uRows.Scan(&id) == nil {
			customerIDs = append(customerIDs, id)
		}
	}
	uRows.Close()

	statuses := []struct {
		Status  string
		Payment string
		Source  string
		Deliv   string
		PayMeth string
	}{
		{"selesai", "paid", "web", "pickup", "pay_at_store"},
		{"selesai", "paid", "web", "delivery", "bank_transfer"},
		{"selesai", "paid", "manual", "pickup", "pay_at_store"},
		{"dikirim", "paid", "web", "delivery", "bank_transfer"},
		{"siap_diambil", "paid", "manual", "pickup", "bank_transfer"},
		{"diproses", "paid", "web", "delivery", "bank_transfer"},
		{"diproses", "unpaid", "web", "pickup", "pay_at_store"},
		{"menunggu_pembayaran", "unpaid", "web", "delivery", "bank_transfer"},
		{"menunggu_konfirmasi", "unpaid", "web", "pickup", "pay_at_store"},
		{"dibatalkan", "unpaid", "web", "delivery", "bank_transfer"},
	}

	now := time.Now()
	for i := 0; i < 30; i++ {
		st := statuses[i%len(statuses)]
		v := variants[i%len(variants)]
		createdAt := now.Add(-time.Duration(29-i) * 20 * time.Hour)
		orderNo := fmt.Sprintf("GMC-%s-%04d", createdAt.Format("060102"), i+1)
		orderID := deterministicUUID("order", orderNo)

		var uID *uuid.UUID
		if len(customerIDs) > 0 {
			id := customerIDs[i%len(customerIDs)]
			uID = &id
		}

		subtotal := v.PriceIdr
		var shippingIdr int64
		if st.Deliv == "delivery" {
			shippingIdr = 35000
		}
		total := subtotal + shippingIdr
		var paidAt *time.Time
		if st.Payment == "paid" {
			t := createdAt.Add(1 * time.Hour)
			paidAt = &t
		}
		addrBytes, _ := json.Marshal(map[string]string{
			"recipientName": "Budi Santoso",
			"phone":         "6281231112233",
			"street":        "Jl. Raya Darmo No. 88",
			"city":          "Surabaya",
			"province":      "Jawa Timur",
			"postalCode":    "60264",
		})
		waURL := platform.BuildOrderWhatsAppURL(platform.WhatsAppOrderParams{
			WhatsAppNumber: "6282232554674",
			StoreName:      "Global Mandiri Computer",
			OrderNo:        orderNo,
			CustomerName:   "Budi Santoso",
			DeliveryMethod: st.Deliv,
			City:           "Surabaya",
			TotalIDR:       total,
			Items: []platform.WhatsAppOrderLine{
				{Name: v.Name, VariantLabel: "Default", Quantity: 1, LineTotalIDR: subtotal},
			},
		})

		_, err := pool.Exec(ctx, `
			INSERT INTO orders (
				id, order_no, user_id, status, payment_status, source,
				customer_name, customer_phone, customer_email,
				delivery_method, shipping_address, payment_method,
				subtotal_idr, discount_idr, shipping_idr, total_idr,
				whatsapp_url, paid_at, created_at, updated_at
			) VALUES (
				$1, $2, $3, $4, $5, $6,
				$7, '6281231112233', 'budi.santoso@example.com',
				$8, $9, $10,
				$11, 0, $12, $13,
				$14, $15, $16, $16
			) ON CONFLICT (order_no) DO NOTHING
		`, orderID, orderNo, uID, st.Status, st.Payment, st.Source,
			fmt.Sprintf("Pelanggan GMC #%d", i+1), st.Deliv, addrBytes, st.PayMeth,
			subtotal, shippingIdr, total, waURL, paidAt, createdAt)
		if err != nil {
			continue
		}

		_, _ = pool.Exec(ctx, `
			INSERT INTO order_items (
				id, order_id, variant_id, product_slug, sku, name, brand,
				variant_label, image_url, unit_price_idr, cost_idr, quantity, line_total_idr
			) VALUES ($1, $2, $3, $4, $5, $6, $7, 'Default', $8, $9, $10, 1, $9)
		`, deterministicUUID("oitem", orderNo), orderID, v.ID, v.Slug, v.SKU, v.Name, v.Brand, v.ImgURL, v.PriceIdr, v.CostIdr)

		_, _ = pool.Exec(ctx, `
			INSERT INTO order_status_history (id, order_id, from_status, to_status, note, actor_id, created_at)
			VALUES ($1, $2, NULL, $3, 'Order dibuat (seed)', $4, $5)
		`, deterministicUUID("ohist", orderNo), orderID, st.Status, adminID, createdAt)
	}

	// Seed Contact Messages & Newsletter Subscribers
	_, _ = pool.Exec(ctx, `
		INSERT INTO contact_messages (id, name, phone, email, subject, message, status)
		VALUES
		  ($1, 'Rizky Pratama', '6281239876543', 'rizky.pratama@example.com', 'rakit_pc', 'Halo GMC, saya ingin konsultasi rakit PC untuk kebutuhan Blender dan Unreal Engine 5 dengan budget Rp 25 juta. Apakah bisa dirakitkan langsung di ITC Mega Grosir hari Sabtu ini?', 'unread'),
		  ($2, 'Hendra Wijaya', '6281132233445', 'hendra.w@example.com', 'stok_harga', 'Selamat siang, apakah Starlink Standard Kit V4 ready stock untuk pengambilan 3 unit kantor proyek di Kalimantan?', 'unread'),
		  ($3, 'Dewi Lestari', '6281703344556', 'dewi.lestari@example.com', 'garansi', 'Terima kasih tim GMC, klaim garansi monitor LG saya sudah dibantu dengan cepat.', 'replied')
		ON CONFLICT (id) DO NOTHING
	`, deterministicUUID("msg", "1"), deterministicUUID("msg", "2"), deterministicUUID("msg", "3"))

	_, _ = pool.Exec(ctx, `
		INSERT INTO newsletter_subscribers (id, email, name)
		VALUES
		  ($1, 'budi.santoso@example.com', 'Budi Santoso'),
		  ($2, 'siti.rahma@example.com', 'Siti Rahma'),
		  ($3, 'arief.hidayat@example.com', 'Arief Hidayat')
		ON CONFLICT (email) DO NOTHING
	`, deterministicUUID("sub", "1"), deterministicUUID("sub", "2"), deterministicUUID("sub", "3"))

	logger.Info("seeded 30 historical orders, contact messages, and newsletter subscribers")
}
