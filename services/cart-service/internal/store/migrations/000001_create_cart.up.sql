CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE carts (
    id uuid primary key default gen_random_uuid(),
    user_id uuid not null unique,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

CREATE TABLE cart_items (
    id uuid primary key default gen_random_uuid(),
    cart_id uuid not null references carts(id) on delete cascade,
    product_id text not null,
    variant_label text not null default '',
    quantity integer not null check (quantity > 0),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (cart_id, product_id, variant_label)
);
