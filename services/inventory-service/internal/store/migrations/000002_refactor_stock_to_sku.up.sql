ALTER TABLE stock
    ADD COLUMN sku_id text;

UPDATE stock
SET sku_id = product_id || '-v1'
WHERE sku_id IS NULL;

ALTER TABLE stock
    ALTER COLUMN sku_id SET NOT NULL,
    DROP CONSTRAINT stock_pkey,
    ADD PRIMARY KEY (sku_id);

CREATE INDEX stock_product_id_idx ON stock (product_id);
