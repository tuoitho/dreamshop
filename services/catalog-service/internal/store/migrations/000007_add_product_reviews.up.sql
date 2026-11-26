CREATE TABLE processed_events (
    event_id text PRIMARY KEY,
    processed_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE review_eligibilities (
    order_id text NOT NULL,
    order_code text NOT NULL DEFAULT '',
    user_id text NOT NULL,
    product_id text NOT NULL,
    sku_id text NOT NULL DEFAULT '',
    variant_label text NOT NULL DEFAULT '',
    customer_name text NOT NULL DEFAULT '',
    delivered_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (order_id, product_id)
);

CREATE INDEX review_eligibilities_user_product_idx
    ON review_eligibilities (user_id, product_id, delivered_at DESC);

CREATE TABLE product_reviews (
    id uuid PRIMARY KEY,
    product_id text NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    user_id text NOT NULL,
    order_id text NOT NULL,
    sku_id text NOT NULL DEFAULT '',
    variant_label text NOT NULL DEFAULT '',
    author_name text NOT NULL,
    rating smallint NOT NULL CHECK (rating BETWEEN 1 AND 5),
    comment text NOT NULL DEFAULT '',
    verified_purchase boolean NOT NULL DEFAULT true,
    hidden boolean NOT NULL DEFAULT false,
    hidden_reason text NOT NULL DEFAULT '',
    hidden_by text NOT NULL DEFAULT '',
    hidden_at timestamptz,
    admin_reply text NOT NULL DEFAULT '',
    admin_reply_by text NOT NULL DEFAULT '',
    admin_replied_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, order_id, product_id)
);

CREATE INDEX product_reviews_product_visible_idx
    ON product_reviews (product_id, created_at DESC)
    WHERE hidden = false;

CREATE INDEX product_reviews_user_product_idx
    ON product_reviews (user_id, product_id, created_at DESC);

CREATE TABLE product_review_images (
    review_id uuid NOT NULL REFERENCES product_reviews(id) ON DELETE CASCADE,
    url text NOT NULL,
    position int NOT NULL,
    PRIMARY KEY (review_id, position)
);

ALTER TABLE product_media_assets
    ADD COLUMN review_id uuid REFERENCES product_reviews(id) ON DELETE SET NULL;

CREATE INDEX product_media_assets_review_idx
    ON product_media_assets (review_id)
    WHERE review_id IS NOT NULL;
