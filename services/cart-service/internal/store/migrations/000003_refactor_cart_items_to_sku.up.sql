DELETE FROM cart_items;

ALTER TABLE cart_items
    DROP CONSTRAINT cart_items_cart_id_product_id_variant_label_key,
    DROP COLUMN variant_label,
    ADD COLUMN sku_id text NOT NULL,
    ADD CONSTRAINT cart_items_sku_id_nonempty CHECK (btrim(sku_id) <> ''),
    ADD CONSTRAINT cart_items_cart_id_sku_id_key UNIQUE (cart_id, sku_id);
