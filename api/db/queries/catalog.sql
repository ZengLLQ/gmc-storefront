-- name: ListPublicCategories :many
SELECT c.*,
  p.slug AS parent_slug,
  (SELECT count(*)::int FROM products pr WHERE pr.category_id = c.id AND pr.is_active = true AND pr.deleted_at IS NULL) AS product_count
FROM categories c
LEFT JOIN categories p ON p.id = c.parent_id
WHERE c.is_active = true
ORDER BY c.sort_order ASC, c.name ASC;

-- name: ListAdminCategories :many
SELECT c.*,
  (SELECT count(*)::int FROM products pr WHERE pr.category_id = c.id AND pr.deleted_at IS NULL) AS product_count
FROM categories c
ORDER BY c.sort_order ASC, c.name ASC;

-- name: GetCategoryBySlug :one
SELECT * FROM categories WHERE slug = $1;

-- name: GetCategoryByID :one
SELECT * FROM categories WHERE id = $1;

-- name: CreateCategory :one
INSERT INTO categories (id, slug, name, parent_id, sort_order, description, image_url, is_active)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateCategory :one
UPDATE categories
SET slug = $2, name = $3, parent_id = $4, sort_order = $5, description = $6, image_url = $7, is_active = $8
WHERE id = $1
RETURNING *;

-- name: UpdateCategorySortOrder :exec
UPDATE categories SET sort_order = $2 WHERE id = $1;

-- name: MoveCategoryProducts :exec
UPDATE products SET category_id = $2 WHERE category_id = $1;

-- name: DeleteCategory :exec
DELETE FROM categories WHERE id = $1;

-- name: ListPublicBrands :many
SELECT b.*,
  (SELECT count(*)::int FROM products pr WHERE pr.brand_id = b.id AND pr.is_active = true AND pr.deleted_at IS NULL) AS product_count
FROM brands b
WHERE b.is_active = true
ORDER BY b.name ASC;

-- name: ListAdminBrands :many
SELECT b.*,
  (SELECT count(*)::int FROM products pr WHERE pr.brand_id = b.id AND pr.deleted_at IS NULL) AS product_count
FROM brands b
ORDER BY b.name ASC;

-- name: GetBrandBySlug :one
SELECT * FROM brands WHERE slug = $1;

-- name: GetBrandByID :one
SELECT * FROM brands WHERE id = $1;

-- name: CreateBrand :one
INSERT INTO brands (id, slug, name, logo_text, is_active)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateBrand :one
UPDATE brands
SET slug = $2, name = $3, logo_text = $4, is_active = $5
WHERE id = $1
RETURNING *;

-- name: DeleteBrand :exec
DELETE FROM brands WHERE id = $1;

-- name: GetProductBySlug :one
SELECT p.*,
  b.slug AS brand_slug, b.name AS brand_name,
  c.slug AS category_slug, c.name AS category_name
FROM products p
JOIN brands b ON b.id = p.brand_id
JOIN categories c ON c.id = p.category_id
WHERE p.slug = $1 AND p.is_active = true AND p.deleted_at IS NULL AND b.is_active = true AND c.is_active = true;

-- name: GetAdminProductByID :one
SELECT * FROM products WHERE id = $1;

-- name: ListProductImages :many
SELECT * FROM product_images WHERE product_id = $1 ORDER BY sort_order ASC;

-- name: ListProductOptions :many
SELECT * FROM product_options WHERE product_id = $1 ORDER BY sort_order ASC;

-- name: ListProductOptionValuesByProduct :many
SELECT pov.*
FROM product_option_values pov
JOIN product_options po ON po.id = pov.option_id
WHERE po.product_id = $1
ORDER BY po.sort_order ASC, pov.sort_order ASC;

-- name: ListActiveVariantsByProduct :many
SELECT v.*, pi.url AS image_url
FROM variants v
LEFT JOIN product_images pi ON pi.id = v.image_id
WHERE v.product_id = $1 AND v.is_active = true
ORDER BY v.price_idr ASC, v.created_at ASC;

-- name: ListAllVariantsByProduct :many
SELECT v.*, pi.url AS image_url
FROM variants v
LEFT JOIN product_images pi ON pi.id = v.image_id
WHERE v.product_id = $1
ORDER BY v.created_at ASC;

-- name: GetVariantBySKU :one
SELECT v.*, p.slug AS product_slug, p.name AS product_name, p.is_active AS product_active, p.deleted_at AS product_deleted_at,
  b.name AS brand_name, pi.url AS image_url
FROM variants v
JOIN products p ON p.id = v.product_id
JOIN brands b ON b.id = p.brand_id
LEFT JOIN product_images pi ON pi.id = v.image_id
WHERE v.sku = $1;

-- name: GetVariantByID :one
SELECT v.*, p.slug AS product_slug, p.name AS product_name, p.is_active AS product_active, p.deleted_at AS product_deleted_at,
  p.category_id, p.brand_id, b.name AS brand_name,
  COALESCE(pi.url, (SELECT url FROM product_images WHERE product_id = p.id ORDER BY sort_order ASC LIMIT 1)) AS resolved_image_url
FROM variants v
JOIN products p ON p.id = v.product_id
JOIN brands b ON b.id = p.brand_id
LEFT JOIN product_images pi ON pi.id = v.image_id
WHERE v.id = $1;

-- name: LockVariantForUpdate :one
SELECT v.*, p.slug AS product_slug, p.name AS product_name, p.is_active AS product_active, p.deleted_at AS product_deleted_at,
  p.category_id, p.brand_id, b.name AS brand_name,
  COALESCE(pi.url, (SELECT url FROM product_images WHERE product_id = p.id ORDER BY sort_order ASC LIMIT 1)) AS resolved_image_url
FROM variants v
JOIN products p ON p.id = v.product_id
JOIN brands b ON b.id = p.brand_id
LEFT JOIN product_images pi ON pi.id = v.image_id
WHERE v.id = $1
FOR UPDATE OF v;

-- name: UpdateVariantStock :exec
UPDATE variants SET stock = $2 WHERE id = $1;

-- name: UpdateVariantCost :exec
UPDATE variants SET cost_idr = $2 WHERE id = $1;

-- name: InsertStockMovement :one
INSERT INTO stock_movements (id, variant_id, delta, stock_after, reason, ref_type, ref_id, note, actor_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: CreateUpload :one
INSERT INTO uploads (id, original_name, mime, bytes, width, height, sha256, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetUploadBySHA256 :one
SELECT * FROM uploads WHERE sha256 = $1 LIMIT 1;

-- name: GetUploadByID :one
SELECT * FROM uploads WHERE id = $1;

-- name: DeleteUpload :exec
DELETE FROM uploads WHERE id = $1;
