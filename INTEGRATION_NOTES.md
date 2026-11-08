# GMC Integration & Architecture Notes

## 1. Monorepo Organization

```
/
├── api/                  # Go 1.26 Backend Service
│   ├── cmd/
│   │   ├── api/main.go   # Go HTTP server entry point (port 8080)
│   │   └── seed/main.go  # Catalog database seeder
│   ├── db/
│   │   ├── migrations/   # Goose SQL migrations (00001_initial.sql, 00002_admin_and_ledger.sql)
│   │   ├── queries/      # sqlc SQL query definitions
│   │   └── seed/         # Seed data (products.csv)
│   ├── internal/
│   │   ├── admin/rbac/   # Admin Role-Based Access Control
│   │   ├── audit/        # Audit logging service
│   │   ├── auth/         # Argon2id + JWT + Refresh Token Rotation
│   │   ├── gen/          # sqlc and oapi-codegen generated Go code
│   │   ├── inventory/    # Stock Ledger with row locks (SELECT FOR UPDATE)
│   │   ├── orders/       # Order management & state machine
│   │   ├── platform/     # Config, DB pool, HTTP helpers, Logger, Cache
│   │   ├── promotions/   # Voucher & discount engine
│   │   └── server/       # Chi HTTP router & handlers
│   └── openapi.yaml      # OpenAPI 3.1 specification for all endpoints
│
└── web/                  # Next.js 15 App Router Storefront
    ├── src/
    │   ├── app/          # App router pages & layouts
    │   │   ├── (store)/  # Customer-facing storefront route group
    │   │   └── api/      # Next.js API proxy to Go backend (preserving cookies)
    │   ├── components/   # Storefront UI components
    │   ├── config/       # site.ts (single source of business truth) & copy.ts
    │   ├── lib/api/      # OpenAPI TypeScript types & API client
    │   └── store/        # Zustand stores (useCartStore, useWishlistStore, useAuthStore)
```

## 2. API Contract & Next.js Proxy

The Next.js storefront communicates with the Go backend through `/api/[...path]/route.ts`.
- Requests to `http://localhost:3000/api/v1/*` are transparently forwarded to `http://localhost:8080/v1/*`.
- Header forwarding ensures `Idempotency-Key`, `Authorization`, and `Cookie` headers pass cleanly between client, proxy, and Go service.
- If the Go API is temporarily unreachable, the frontend gracefully falls back to cached/seeded product collections so the storefront remains resilient.

## 3. Database Schema Highlights

- **`categories` & `brands`:** Structured catalog taxonomy with URL-friendly slugs.
- **`products` & `variants`:** Multi-variant support with JSONB attribute maps and SKU tracking.
- **`inventory_ledger`:** Immutable double-entry inventory ledger tracking every stock adjustment, restock, checkout deduction, and reservation.
- **`orders` & `order_items`:** Idempotent checkout handling with unique `order_no` format (e.g. `GMC-20261016-XXXX`).
- **`site_settings`, `announcements`, `promo_slides`, `faqs`:** Dynamic CMS data editable by future admin panel and consumed by the storefront.
