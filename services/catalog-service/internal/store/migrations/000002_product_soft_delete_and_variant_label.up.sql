ALTER TABLE products ADD COLUMN deleted_at timestamptz;

ALTER TABLE product_variants RENAME COLUMN color TO label;
ALTER TABLE product_variants ALTER COLUMN color_hex DROP NOT NULL;

CREATE INDEX products_active_idx ON products (id) WHERE deleted_at IS NULL;
