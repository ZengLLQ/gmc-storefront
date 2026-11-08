-- name: UpsertUserCart :exec
INSERT INTO carts (user_id, updated_at)
VALUES ($1, now())
ON CONFLICT (user_id) DO UPDATE SET updated_at = now();

-- name: ClearUserCartItems :exec
DELETE FROM cart_items WHERE user_id = $1;

-- name: UpsertCartItem :exec
INSERT INTO cart_items (user_id, variant_id, quantity)
VALUES ($1, $2, $3)
ON CONFLICT (user_id, variant_id) DO UPDATE SET quantity = EXCLUDED.quantity;

-- name: ListUserCartItems :many
SELECT variant_id, quantity FROM cart_items WHERE user_id = $1;

-- name: NextOrderSequence :one
INSERT INTO order_counters (day, last_seq)
VALUES ($1, 1)
ON CONFLICT (day) DO UPDATE SET last_seq = order_counters.last_seq + 1
RETURNING last_seq;

-- name: GetIdempotencyKey :one
SELECT * FROM idempotency_keys WHERE key = $1;

-- name: SaveIdempotencyKey :exec
INSERT INTO idempotency_keys (key, request_hash, response, status_code)
VALUES ($1, $2, $3, $4)
ON CONFLICT (key) DO NOTHING;

-- name: CreateOrder :one
INSERT INTO orders (
  id, order_no, user_id, customer_name, customer_phone, customer_email,
  delivery_method, shipping_address, payment_method, note,
  subtotal_idr, discount_idr, shipping_idr, total_idr,
  status, payment_status, paid_at, voucher_code, source, internal_note,
  view_token_hash, idempotency_key, request_hash
) VALUES (
  $1, $2, $3, $4, $5, $6,
  $7, $8, $9, $10,
  $11, $12, $13, $14,
  $15, $16, $17, $18, $19, $20,
  $21, $22, $23
)
RETURNING *;

-- name: CreateOrderItem :one
INSERT INTO order_items (
  id, order_id, variant_id, product_slug, sku, name, brand,
  variant_label, image_url, unit_price_idr, cost_idr, quantity, line_total_idr
) VALUES (
  $1, $2, $3, $4, $5, $6, $7,
  $8, $9, $10, $11, $12, $13
)
RETURNING *;

-- name: DeleteOrderItemsByOrderID :exec
DELETE FROM order_items WHERE order_id = $1;

-- name: CreateOrderStatusHistory :one
INSERT INTO order_status_history (id, order_id, status, note)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetOrderByOrderNo :one
SELECT * FROM orders WHERE order_no = $1;

-- name: LockOrderByOrderNo :one
SELECT * FROM orders WHERE order_no = $1 FOR UPDATE;

-- name: ListOrderItemsByOrderID :many
SELECT * FROM order_items WHERE order_id = $1 ORDER BY id ASC;

-- name: ListOrderStatusHistoryByOrderID :many
SELECT * FROM order_status_history WHERE order_id = $1 ORDER BY created_at ASC;

-- name: UpdateOrderStatus :one
UPDATE orders
SET status = $2,
    cancel_reason = COALESCE(sqlc.narg('cancel_reason'), cancel_reason),
    version = version + 1,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateOrderMeta :one
UPDATE orders
SET shipping_idr = $2,
    total_idr = subtotal_idr - discount_idr + $2,
    courier = $3,
    tracking_no = $4,
    payment_status = $5,
    paid_at = $6,
    internal_note = $7,
    version = version + 1,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: GetVoucherByCode :one
SELECT * FROM vouchers WHERE code = $1;

-- name: LockVoucherByCode :one
SELECT * FROM vouchers WHERE code = $1 FOR UPDATE;

-- name: IncrementVoucherUsedCount :exec
UPDATE vouchers SET used_count = used_count + 1 WHERE id = $1;

-- name: DecrementVoucherUsedCount :exec
UPDATE vouchers SET used_count = GREATEST(0, used_count - 1) WHERE code = $1;

-- name: CountVoucherRedemptionsByUser :one
SELECT count(*) FROM voucher_redemptions
WHERE voucher_id = $1 AND (
  (sqlc.narg('user_id')::uuid IS NOT NULL AND user_id = sqlc.narg('user_id')::uuid)
  OR (phone = $2)
);

-- name: CreateVoucherRedemption :exec
INSERT INTO voucher_redemptions (id, voucher_id, order_id, user_id, phone, discount_idr)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: DeleteVoucherRedemptionByOrder :exec
DELETE FROM voucher_redemptions WHERE order_id = $1;
