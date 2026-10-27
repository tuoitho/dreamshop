ALTER TABLE product_media_assets
    ADD COLUMN IF NOT EXISTS sweep_locked_until timestamptz;
