ALTER TABLE products
    ADD COLUMN max_price_amount_minor bigint NOT NULL DEFAULT 0;

UPDATE products
SET max_price_amount_minor = price_amount_minor
WHERE max_price_amount_minor = 0;

ALTER TABLE product_variants
    ADD COLUMN sku_id text,
    ADD COLUMN price_amount_minor bigint,
    ADD COLUMN original_price_amount_minor bigint;

UPDATE product_variants pv
SET sku_id = pv.product_id || '-v' || (pv.position + 1)::text,
    price_amount_minor = p.price_amount_minor,
    original_price_amount_minor = p.original_price_amount_minor
FROM products p
WHERE p.id = pv.product_id;

ALTER TABLE product_variants
    ALTER COLUMN sku_id SET NOT NULL,
    ALTER COLUMN price_amount_minor SET NOT NULL,
    ALTER COLUMN original_price_amount_minor SET NOT NULL,
    DROP CONSTRAINT product_variants_pkey,
    ADD PRIMARY KEY (sku_id),
    ADD CONSTRAINT product_variants_price_positive CHECK (price_amount_minor > 0),
    ADD CONSTRAINT product_variants_original_price_valid CHECK (
        original_price_amount_minor >= price_amount_minor
    );

CREATE INDEX product_variants_product_pos_idx
    ON product_variants (product_id, position);
