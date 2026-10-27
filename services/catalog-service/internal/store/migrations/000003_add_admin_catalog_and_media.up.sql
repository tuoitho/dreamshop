ALTER TABLE products
    ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();

CREATE TABLE product_videos (
    product_id text REFERENCES products(id) ON DELETE CASCADE,
    url text NOT NULL,
    position int NOT NULL,
    PRIMARY KEY (product_id, position)
);

CREATE TABLE product_media_assets (
    id uuid PRIMARY KEY,
    bucket text NOT NULL,
    object_key text NOT NULL UNIQUE,
    public_url text NOT NULL UNIQUE,
    media_type text NOT NULL CHECK (media_type IN ('IMAGE', 'VIDEO')),
    original_filename text NOT NULL DEFAULT '',
    content_type text NOT NULL,
    size_bytes bigint NOT NULL CHECK (size_bytes > 0),
    status text NOT NULL CHECK (status IN ('PENDING', 'ATTACHED', 'ORPHANED')),
    uploaded_by text NOT NULL,
    product_id text REFERENCES products(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    attached_at timestamptz,
    expires_at timestamptz
);

CREATE INDEX product_media_assets_sweep_idx
    ON product_media_assets (expires_at)
    WHERE status IN ('PENDING', 'ORPHANED');

CREATE INDEX product_media_assets_product_idx
    ON product_media_assets (product_id)
    WHERE product_id IS NOT NULL;
