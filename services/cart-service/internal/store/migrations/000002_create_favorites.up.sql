CREATE TABLE favorite_items (
    id uuid primary key default gen_random_uuid(),
    user_id uuid not null,
    product_id text not null,
    created_at timestamptz not null default now(),
    unique (user_id, product_id)
);

CREATE INDEX ix_favorite_items_user_created
    ON favorite_items (user_id, created_at desc, id desc);
