-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS unaccent;
CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
  id uuid PRIMARY KEY,
  email citext UNIQUE NOT NULL,
  name text NOT NULL,
  phone text NOT NULL,
  password_hash text NOT NULL,
  role text NOT NULL DEFAULT 'customer' CHECK (role IN ('customer','admin')),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  family_id uuid NOT NULL,
  token_hash bytea UNIQUE NOT NULL,
  rotated_at timestamptz,
  replaced_by uuid,
  revoked_at timestamptz,
  user_agent text,
  ip inet,
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_family_id ON sessions(family_id);

CREATE TABLE addresses (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  label text,
  recipient_name text NOT NULL,
  phone text NOT NULL,
  street text NOT NULL,
  city text NOT NULL,
  province text NOT NULL,
  postal_code text NOT NULL,
  is_default boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX one_default_address ON addresses(user_id) WHERE is_default;

CREATE TABLE categories (
  id uuid PRIMARY KEY,
  slug text UNIQUE NOT NULL,
  name text NOT NULL,
  parent_id uuid REFERENCES categories(id),
  sort_order int NOT NULL DEFAULT 0,
  description text
);

CREATE TABLE brands (
  id uuid PRIMARY KEY,
  slug text UNIQUE NOT NULL,
  name text NOT NULL
);

CREATE TABLE products (
  id uuid PRIMARY KEY,
  slug text UNIQUE NOT NULL,
  name text NOT NULL,
  brand_id uuid NOT NULL REFERENCES brands(id),
  category_id uuid NOT NULL REFERENCES categories(id),
  summary text NOT NULL DEFAULT '',
  description text NOT NULL DEFAULT '',
  highlights text[] NOT NULL DEFAULT '{}',
  specs jsonb NOT NULL DEFAULT '{}',
  is_featured boolean NOT NULL DEFAULT false,
  is_active boolean NOT NULL DEFAULT true,
  is_demo boolean NOT NULL DEFAULT false,
  search_vector tsvector GENERATED ALWAYS AS (
    setweight(to_tsvector('simple', coalesce(name,'')),'A') ||
    setweight(to_tsvector('simple', coalesce(summary,'')),'B')
  ) STORED,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_products_category_active ON products(category_id) WHERE is_active;
CREATE INDEX idx_products_brand_active ON products(brand_id) WHERE is_active;
CREATE INDEX idx_products_search_vector ON products USING gin(search_vector);
CREATE INDEX idx_products_name_trgm ON products USING gin(name gin_trgm_ops);

CREATE TABLE product_images (
  id uuid PRIMARY KEY,
  product_id uuid NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  url text NOT NULL,
  alt text NOT NULL DEFAULT '',
  width int,
  height int,
  sort_order int NOT NULL DEFAULT 0,
  kind text NOT NULL DEFAULT 'product' CHECK (kind IN ('product','lifestyle','detail'))
);
CREATE INDEX idx_product_images_product_sort ON product_images(product_id, sort_order);

CREATE TABLE product_options (
  id uuid PRIMARY KEY,
  product_id uuid NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  name text NOT NULL,
  sort_order int NOT NULL DEFAULT 0,
  display text NOT NULL DEFAULT 'pill' CHECK (display IN ('pill','swatch'))
);

CREATE TABLE product_option_values (
  id uuid PRIMARY KEY,
  option_id uuid NOT NULL REFERENCES product_options(id) ON DELETE CASCADE,
  label text NOT NULL,
  swatch_hex text,
  sort_order int NOT NULL DEFAULT 0
);

CREATE TABLE variants (
  id uuid PRIMARY KEY,
  product_id uuid NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  sku text UNIQUE NOT NULL,
  option_value_ids uuid[] NOT NULL DEFAULT '{}',
  price_idr bigint NOT NULL CHECK (price_idr >= 0),
  compare_at_price_idr bigint CHECK (compare_at_price_idr IS NULL OR compare_at_price_idr > price_idr),
  stock int NOT NULL DEFAULT 0 CHECK (stock >= 0),
  is_preorder boolean NOT NULL DEFAULT false,
  is_active boolean NOT NULL DEFAULT true,
  image_id uuid REFERENCES product_images(id),
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_variants_product_active ON variants(product_id) WHERE is_active;
CREATE INDEX idx_variants_price_idr ON variants(price_idr);

CREATE TABLE related_products (
  product_id uuid REFERENCES products(id) ON DELETE CASCADE,
  related_id uuid REFERENCES products(id) ON DELETE CASCADE,
  sort_order int NOT NULL DEFAULT 0,
  PRIMARY KEY(product_id, related_id)
);

CREATE TABLE carts (
  user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE cart_items (
  user_id uuid REFERENCES users(id) ON DELETE CASCADE,
  variant_id uuid REFERENCES variants(id) ON DELETE CASCADE,
  quantity int NOT NULL CHECK (quantity BETWEEN 1 AND 99),
  PRIMARY KEY(user_id, variant_id)
);

CREATE TABLE order_counters (
  day date PRIMARY KEY,
  last_seq int NOT NULL
);

CREATE TABLE orders (
  id uuid PRIMARY KEY,
  order_no text UNIQUE NOT NULL,
  user_id uuid REFERENCES users(id) ON DELETE SET NULL,
  customer_name text NOT NULL,
  customer_phone text NOT NULL,
  customer_email citext NOT NULL,
  delivery_method text NOT NULL CHECK (delivery_method IN ('pickup','delivery')),
  shipping_address jsonb,
  payment_method text NOT NULL CHECK (payment_method IN ('bank_transfer','cod','pay_at_store')),
  note text,
  subtotal_idr bigint NOT NULL,
  shipping_idr bigint NOT NULL DEFAULT 0,
  total_idr bigint NOT NULL,
  status text NOT NULL DEFAULT 'menunggu_konfirmasi' CHECK (status IN
    ('menunggu_konfirmasi','menunggu_pembayaran','diproses','dikirim','siap_diambil','selesai','dibatalkan')),
  view_token_hash bytea NOT NULL,
  idempotency_key uuid,
  request_hash bytea,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_orders_user_created ON orders(user_id, created_at DESC);
CREATE UNIQUE INDEX idx_orders_idempotency_key ON orders(idempotency_key) WHERE idempotency_key IS NOT NULL;

CREATE TABLE order_items (
  id uuid PRIMARY KEY,
  order_id uuid NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  variant_id uuid REFERENCES variants(id) ON DELETE SET NULL,
  product_slug text NOT NULL,
  sku text NOT NULL,
  name text NOT NULL,
  brand text NOT NULL,
  variant_label text NOT NULL DEFAULT '',
  image_url text,
  unit_price_idr bigint NOT NULL,
  quantity int NOT NULL CHECK (quantity > 0),
  line_total_idr bigint NOT NULL
);
CREATE INDEX idx_order_items_order_id ON order_items(order_id);

CREATE TABLE order_status_history (
  id uuid PRIMARY KEY,
  order_id uuid NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  status text NOT NULL,
  note text,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_order_status_history_order_id ON order_status_history(order_id, created_at ASC);

CREATE TABLE idempotency_keys (
  key uuid PRIMARY KEY,
  request_hash bytea NOT NULL,
  response jsonb NOT NULL,
  status_code int NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE wishlist_items (
  user_id uuid REFERENCES users(id) ON DELETE CASCADE,
  product_id uuid REFERENCES products(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY(user_id, product_id)
);

CREATE TABLE contact_messages (
  id uuid PRIMARY KEY,
  name text NOT NULL,
  email citext NOT NULL,
  phone text NOT NULL,
  subject text NOT NULL,
  message text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE newsletter_subscribers (
  id uuid PRIMARY KEY,
  email citext UNIQUE NOT NULL,
  name text,
  created_at timestamptz NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS newsletter_subscribers;
DROP TABLE IF EXISTS contact_messages;
DROP TABLE IF EXISTS wishlist_items;
DROP TABLE IF EXISTS idempotency_keys;
DROP TABLE IF EXISTS order_status_history;
DROP TABLE IF EXISTS order_items;
DROP TABLE IF EXISTS orders;
DROP TABLE IF EXISTS order_counters;
DROP TABLE IF EXISTS cart_items;
DROP TABLE IF EXISTS carts;
DROP TABLE IF EXISTS related_products;
DROP TABLE IF EXISTS variants;
DROP TABLE IF EXISTS product_option_values;
DROP TABLE IF EXISTS product_options;
DROP TABLE IF EXISTS product_images;
DROP TABLE IF EXISTS products;
DROP TABLE IF EXISTS brands;
DROP TABLE IF EXISTS categories;
DROP TABLE IF EXISTS addresses;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
