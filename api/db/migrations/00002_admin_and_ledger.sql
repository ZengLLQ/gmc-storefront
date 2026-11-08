-- +goose Up
-- +goose StatementBegin
ALTER TABLE users DROP CONSTRAINT users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('customer','staff','admin'));
ALTER TABLE users ADD COLUMN is_active boolean NOT NULL DEFAULT true,
  ADD COLUMN last_login_at timestamptz,
  ADD COLUMN must_change_password boolean NOT NULL DEFAULT false,
  ADD COLUMN internal_note text;

ALTER TABLE sessions ADD COLUMN is_admin_session boolean NOT NULL DEFAULT false,
  ADD COLUMN absolute_expires_at timestamptz;

CREATE TABLE user_totp (
  user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  secret_enc bytea NOT NULL,
  confirmed_at timestamptz,
  backup_codes_hash bytea[] NOT NULL DEFAULT '{}'
);

ALTER TABLE products ADD COLUMN deleted_at timestamptz,
  ADD COLUMN version int NOT NULL DEFAULT 1,
  ADD COLUMN seo_title text,
  ADD COLUMN seo_description text;

ALTER TABLE variants ADD COLUMN cost_idr bigint CHECK (cost_idr IS NULL OR cost_idr >= 0),
  ADD COLUMN low_stock_threshold int NOT NULL DEFAULT 3 CHECK (low_stock_threshold >= 0),
  ADD COLUMN barcode text;

ALTER TABLE categories ADD COLUMN image_url text,
  ADD COLUMN is_active boolean NOT NULL DEFAULT true;

ALTER TABLE brands ADD COLUMN logo_text text,
  ADD COLUMN is_active boolean NOT NULL DEFAULT true;

ALTER TABLE product_images ADD COLUMN upload_id uuid;

CREATE TABLE uploads (
  id uuid PRIMARY KEY,
  original_name text NOT NULL,
  mime text NOT NULL,
  bytes int NOT NULL,
  width int NOT NULL,
  height int NOT NULL,
  sha256 bytea NOT NULL,
  created_by uuid REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_uploads_sha256 ON uploads(sha256);

CREATE TABLE stock_movements (
  id uuid PRIMARY KEY,
  variant_id uuid NOT NULL REFERENCES variants(id) ON DELETE CASCADE,
  delta int NOT NULL CHECK (delta <> 0),
  stock_after int NOT NULL CHECK (stock_after >= 0),
  reason text NOT NULL CHECK (reason IN ('order','order_cancel','order_edit','restock','correction','damaged','return','initial','import')),
  ref_type text,
  ref_id uuid,
  note text,
  actor_id uuid REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_stock_movements_variant_created ON stock_movements(variant_id, created_at DESC);
CREATE INDEX idx_stock_movements_created ON stock_movements(created_at DESC);

-- Backfill initial stock movements for any existing variants with stock > 0
INSERT INTO stock_movements (id, variant_id, delta, stock_after, reason, note)
SELECT gen_random_uuid(), id, stock, stock, 'initial', 'Backfill initial stock'
FROM variants
WHERE stock > 0;

ALTER TABLE orders ADD COLUMN discount_idr bigint NOT NULL DEFAULT 0 CHECK (discount_idr >= 0),
  ADD COLUMN voucher_code text,
  ADD COLUMN courier text,
  ADD COLUMN tracking_no text,
  ADD COLUMN payment_status text NOT NULL DEFAULT 'unpaid' CHECK (payment_status IN ('unpaid','paid','refunded')),
  ADD COLUMN paid_at timestamptz,
  ADD COLUMN internal_note text,
  ADD COLUMN source text NOT NULL DEFAULT 'web' CHECK (source IN ('web','manual')),
  ADD COLUMN cancel_reason text,
  ADD COLUMN version int NOT NULL DEFAULT 1;

ALTER TABLE orders ADD CONSTRAINT orders_total_ck CHECK (total_idr = subtotal_idr - discount_idr + shipping_idr);
CREATE INDEX idx_orders_status_created ON orders(status, created_at DESC);
CREATE INDEX idx_orders_payment_status ON orders(payment_status);
CREATE INDEX idx_orders_customer_phone ON orders(customer_phone);
CREATE INDEX idx_orders_customer_name_trgm ON orders USING gin(customer_name gin_trgm_ops);

ALTER TABLE order_items ADD COLUMN cost_idr bigint;

CREATE TABLE vouchers (
  id uuid PRIMARY KEY,
  code citext UNIQUE NOT NULL,
  description text NOT NULL DEFAULT '',
  kind text NOT NULL CHECK (kind IN ('percent','fixed')),
  value int NOT NULL CHECK (value > 0),
  max_discount_idr bigint,
  min_subtotal_idr bigint NOT NULL DEFAULT 0,
  usage_limit int,
  per_user_limit int NOT NULL DEFAULT 1,
  used_count int NOT NULL DEFAULT 0,
  starts_at timestamptz,
  ends_at timestamptz,
  is_active boolean NOT NULL DEFAULT true,
  scope_category_ids uuid[] NOT NULL DEFAULT '{}',
  scope_brand_ids uuid[] NOT NULL DEFAULT '{}',
  version int NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE voucher_redemptions (
  id uuid PRIMARY KEY,
  voucher_id uuid NOT NULL REFERENCES vouchers(id) ON DELETE CASCADE,
  order_id uuid NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  user_id uuid,
  phone text,
  discount_idr bigint NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_voucher_redemptions_voucher_id ON voucher_redemptions(voucher_id, created_at DESC);

CREATE TABLE site_settings (
  key text PRIMARY KEY,
  value jsonb NOT NULL,
  version int NOT NULL DEFAULT 1,
  updated_by uuid REFERENCES users(id),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE announcements (
  id uuid PRIMARY KEY,
  text text NOT NULL,
  link_url text,
  sort_order int NOT NULL DEFAULT 0,
  is_active boolean NOT NULL DEFAULT true,
  starts_at timestamptz,
  ends_at timestamptz
);

CREATE TABLE promo_slides (
  id uuid PRIMARY KEY,
  label text,
  headline text NOT NULL,
  body text,
  cta_label text,
  cta_url text,
  image_url text,
  image_mobile_url text,
  image_upload_id uuid REFERENCES uploads(id),
  image_mobile_upload_id uuid REFERENCES uploads(id),
  sort_order int NOT NULL DEFAULT 0,
  is_active boolean NOT NULL DEFAULT true,
  starts_at timestamptz,
  ends_at timestamptz
);

CREATE TABLE faqs (
  id uuid PRIMARY KEY,
  anchor text NOT NULL,
  question text NOT NULL,
  answer text NOT NULL,
  sort_order int NOT NULL DEFAULT 0,
  is_active boolean NOT NULL DEFAULT true
);

ALTER TABLE contact_messages ADD COLUMN status text NOT NULL DEFAULT 'new' CHECK (status IN ('new','read','replied','archived')),
  ADD COLUMN internal_note text,
  ADD COLUMN handled_by uuid REFERENCES users(id),
  ADD COLUMN handled_at timestamptz;

CREATE TABLE audit_log (
  id uuid PRIMARY KEY,
  at timestamptz NOT NULL DEFAULT now(),
  actor_id uuid REFERENCES users(id),
  actor_email text NOT NULL,
  actor_role text NOT NULL,
  action text NOT NULL,
  entity_type text NOT NULL,
  entity_id text,
  summary text NOT NULL,
  before jsonb,
  after jsonb,
  ip inet,
  request_id text
);
CREATE INDEX idx_audit_log_at ON audit_log(at DESC);
CREATE INDEX idx_audit_log_entity ON audit_log(entity_type, entity_id);
CREATE INDEX idx_audit_log_actor ON audit_log(actor_id, at DESC);

CREATE FUNCTION audit_log_immutable() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'audit_log is append-only';
END
$$ LANGUAGE plpgsql;

CREATE TRIGGER audit_log_no_update
BEFORE UPDATE OR DELETE ON audit_log
FOR EACH ROW EXECUTE FUNCTION audit_log_immutable();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS audit_log_no_update ON audit_log;
DROP FUNCTION IF EXISTS audit_log_immutable();
DROP TABLE IF EXISTS audit_log;

ALTER TABLE contact_messages
  DROP COLUMN IF EXISTS status,
  DROP COLUMN IF EXISTS internal_note,
  DROP COLUMN IF EXISTS handled_by,
  DROP COLUMN IF EXISTS handled_at;

DROP TABLE IF EXISTS faqs;
DROP TABLE IF EXISTS promo_slides;
DROP TABLE IF EXISTS announcements;
DROP TABLE IF EXISTS site_settings;
DROP TABLE IF EXISTS voucher_redemptions;
DROP TABLE IF EXISTS vouchers;

ALTER TABLE order_items DROP COLUMN IF EXISTS cost_idr;

DROP INDEX IF EXISTS idx_orders_customer_name_trgm;
DROP INDEX IF EXISTS idx_orders_customer_phone;
DROP INDEX IF EXISTS idx_orders_payment_status;
DROP INDEX IF EXISTS idx_orders_status_created;
ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_total_ck;
ALTER TABLE orders
  DROP COLUMN IF EXISTS discount_idr,
  DROP COLUMN IF EXISTS voucher_code,
  DROP COLUMN IF EXISTS courier,
  DROP COLUMN IF EXISTS tracking_no,
  DROP COLUMN IF EXISTS payment_status,
  DROP COLUMN IF EXISTS paid_at,
  DROP COLUMN IF EXISTS internal_note,
  DROP COLUMN IF EXISTS source,
  DROP COLUMN IF EXISTS cancel_reason,
  DROP COLUMN IF EXISTS version;

DROP TABLE IF EXISTS stock_movements;
DROP TABLE IF EXISTS uploads;

ALTER TABLE product_images DROP COLUMN IF EXISTS upload_id;
ALTER TABLE brands DROP COLUMN IF EXISTS logo_text, DROP COLUMN IF EXISTS is_active;
ALTER TABLE categories DROP COLUMN IF EXISTS image_url, DROP COLUMN IF EXISTS is_active;
ALTER TABLE variants DROP COLUMN IF EXISTS cost_idr, DROP COLUMN IF EXISTS low_stock_threshold, DROP COLUMN IF EXISTS barcode;
ALTER TABLE products DROP COLUMN IF EXISTS deleted_at, DROP COLUMN IF EXISTS version, DROP COLUMN IF EXISTS seo_title, DROP COLUMN IF EXISTS seo_description;

DROP TABLE IF EXISTS user_totp;
ALTER TABLE sessions DROP COLUMN IF EXISTS is_admin_session, DROP COLUMN IF EXISTS absolute_expires_at;

UPDATE users SET role = 'customer' WHERE role = 'staff';
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('customer','admin'));
ALTER TABLE users
  DROP COLUMN IF EXISTS is_active,
  DROP COLUMN IF EXISTS last_login_at,
  DROP COLUMN IF EXISTS must_change_password,
  DROP COLUMN IF EXISTS internal_note;
-- +goose StatementEnd
