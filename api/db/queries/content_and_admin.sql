-- name: AddWishlistItem :exec
INSERT INTO wishlist_items (user_id, product_id)
VALUES ($1, $2)
ON CONFLICT (user_id, product_id) DO NOTHING;

-- name: RemoveWishlistItem :exec
DELETE FROM wishlist_items WHERE user_id = $1 AND product_id = $2;

-- name: ListUserWishlistProductIDs :many
SELECT product_id FROM wishlist_items WHERE user_id = $1 ORDER BY created_at DESC;

-- name: CreateContactMessage :one
INSERT INTO contact_messages (id, name, email, phone, subject, message)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpsertNewsletterSubscriber :exec
INSERT INTO newsletter_subscribers (id, email, name)
VALUES ($1, $2, $3)
ON CONFLICT (email) DO UPDATE SET name = COALESCE(EXCLUDED.name, newsletter_subscribers.name);

-- name: GetSiteSetting :one
SELECT * FROM site_settings WHERE key = $1;

-- name: ListSiteSettings :many
SELECT * FROM site_settings ORDER BY key ASC;

-- name: UpsertSiteSetting :one
INSERT INTO site_settings (key, value, version, updated_by, updated_at)
VALUES ($1, $2, 1, $3, now())
ON CONFLICT (key) DO UPDATE
SET value = EXCLUDED.value,
    version = site_settings.version + 1,
    updated_by = EXCLUDED.updated_by,
    updated_at = now()
RETURNING *;

-- name: ListActiveAnnouncements :many
SELECT * FROM announcements
WHERE is_active = true
  AND (starts_at IS NULL OR starts_at <= now())
  AND (ends_at IS NULL OR ends_at >= now())
ORDER BY sort_order ASC;

-- name: ListAllAnnouncements :many
SELECT * FROM announcements ORDER BY sort_order ASC;

-- name: CreateAnnouncement :one
INSERT INTO announcements (id, text, link_url, sort_order, is_active, starts_at, ends_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: UpdateAnnouncement :one
UPDATE announcements
SET text = $2, link_url = $3, sort_order = $4, is_active = $5, starts_at = $6, ends_at = $7
WHERE id = $1
RETURNING *;

-- name: DeleteAnnouncement :exec
DELETE FROM announcements WHERE id = $1;

-- name: ListActivePromoSlides :many
SELECT * FROM promo_slides
WHERE is_active = true
  AND (starts_at IS NULL OR starts_at <= now())
  AND (ends_at IS NULL OR ends_at >= now())
ORDER BY sort_order ASC;

-- name: ListAllPromoSlides :many
SELECT * FROM promo_slides ORDER BY sort_order ASC;

-- name: CreatePromoSlide :one
INSERT INTO promo_slides (id, label, headline, body, cta_label, cta_url, image_url, image_mobile_url, sort_order, is_active)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: UpdatePromoSlide :one
UPDATE promo_slides
SET label = $2, headline = $3, body = $4, cta_label = $5, cta_url = $6, image_url = $7, image_mobile_url = $8, sort_order = $9, is_active = $10
WHERE id = $1
RETURNING *;

-- name: DeletePromoSlide :exec
DELETE FROM promo_slides WHERE id = $1;

-- name: ListActiveFaqs :many
SELECT * FROM faqs WHERE is_active = true ORDER BY sort_order ASC;

-- name: ListAllFaqs :many
SELECT * FROM faqs ORDER BY sort_order ASC;

-- name: CreateFaq :one
INSERT INTO faqs (id, anchor, question, answer, sort_order, is_active)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateFaq :one
UPDATE faqs
SET anchor = $2, question = $3, answer = $4, sort_order = $5, is_active = $6
WHERE id = $1
RETURNING *;

-- name: DeleteFaq :exec
DELETE FROM faqs WHERE id = $1;

-- name: InsertAuditLog :one
INSERT INTO audit_log (
  id, actor_id, actor_email, actor_role, action, entity_type, entity_id,
  summary, before, after, ip, request_id
) VALUES (
  $1, $2, $3, $4, $5, $6, $7,
  $8, $9, $10, $11::inet, $12
)
RETURNING *;
