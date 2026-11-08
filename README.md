# Global Mandiri Computer (GMC) — E-Commerce Storefront

Modern, high-performance, customer-facing e-commerce storefront for **Global Mandiri Computer (GMC)**, an IT hardware & custom PC store located at **ITC Mega Grosir Lt. 2 Blok B/7, Surabaya**.

Designed following a minimalist, luxury design language (inspired by Lujo Living) with warm cream backgrounds, photographic product stages, hairline borders, and pill buttons. All user copy is in **Bahasa Indonesia** with integer Rupiah formatting (`Rp 1.234.567`).

---

## 🚀 System Architecture & Stack

- **Frontend:** Next.js 15 (App Router), TypeScript (Strict), Tailwind CSS v4, Framer Motion, Lenis Smooth Scroll, Zustand, TanStack Query v5, Lucide Icons.
- **Backend:** Go 1.26 RESTful API, Chi Router, OpenAPI 3.1 specification (`oapi-codegen`), PostgreSQL (`sqlc` type-safe queries), Goose migrations.
- **Database:** PostgreSQL with transactional orders, row-level locking (`SELECT FOR UPDATE`), inventory ledger audit trails, and strict check constraints.

---

## 🌐 Running Services

| Service | Port / URL | Description |
|---|---|---|
| **Storefront Web** | [http://localhost:3000](http://localhost:3000) | Next.js Customer Storefront & API Proxy |
| **Go REST API** | [http://localhost:8080](http://localhost:8080) | Go Backend (`/v1/*` & `/v1/admin/*`) |
| **PostgreSQL** | `localhost:5432` | Docker container `gmc-postgres` (`user=gmc_user`, `db=gmc_db`) |

---

## 📁 Key Routes & Pages

- **`/` (Beranda):** Rotating announcement bar, Ken-Burns hero, interactive category showcase with hover-synced media, filterable "Produk Pilihan" grid, dual promo slideshows, "Kenapa GMC" 5 pillars with synced tabs, infinite brand marquee, and ITC Mega Grosir visit information.
- **`/shop` & `/shop/c/[category]`:** Product catalog with real-time text search, category facet filters, brand filters, in-stock toggle, dual price range slider, sorting (terbaru, harga termurah, termahal, terpopuler), and pagination.
- **`/shop/[slug]`:** Product Detail Page with snap peek gallery, interactive variant selector, stock status, specs breakdown, cross-sell recommendations, WhatsApp consultation link, and JSON-LD structured data.
- **`/cart` & Cart Drawer:** Global cart slide-in drawer on add-to-cart, quantity modifiers, subtotal calculations, and full cart page.
- **`/checkout`:** Multi-step checkout with customer info, delivery options (Ambil di Toko ITC Surabaya or Kurir Ekspedisi), payment instructions, promo voucher code input, and order summary.
- **`/checkout/success/[orderNo]`:** Order confirmation with order breakdown, payment transfer instructions, and direct WhatsApp confirmation button.
- **`/about`:** Store origin, values, and photo gallery of the physical store at ITC Mega Grosir Surabaya.
- **`/contact`:** Store contact details, opening hours, Google Maps link, WhatsApp direct link, message inquiry form, and interactive expandable FAQ accordion.
- **`/account`:** Customer account dashboard with order history, address book, profile details, and wishlist.
- **`/login` & `/register`:** Customer authentication forms.
- **`/privacy` & `/terms`:** Privacy policy and terms of service.

---

## 🛠️ Quick Start & Commands

### 1. Database
```bash
docker start gmc-postgres
```

### 2. Run Database Migrations & Seeder
```bash
cd api
goose -dir db/migrations postgres "postgres://gmc_user:gmc_pass@localhost:5432/gmc_db?sslmode=disable" up
go run cmd/seed/main.go
```

### 3. Start Backend API
```bash
cd api
go run cmd/api/main.go
# API is available at http://localhost:8080
```

### 4. Start Next.js Storefront
```bash
cd web
npm run dev
# Storefront is available at http://localhost:3000
```

---

## 🎨 Design Tokens

- Page Background: `#FBF7EF` (warm cream)
- Card / Stage Panel: `#F2EFE8` (neutral warm gray)
- Footer Background: `#F3F0E9`
- Hairline Dividers: `1px solid #DAD6CC`
- Primary Ink: `#111111`
- Secondary Ink: `#5F5C55`
- Primary Button: Fully rounded pill (`border-radius: 999px`), solid black, white micro-label
- Product Stage Radius: Square (`border-radius: 0px`)
- Font: Inter (Weights 400 & 500)
