CREATE TABLE stock (
    product_id text primary key,
    quantity integer not null check (quantity >= 0),
    reserved integer not null default 0 check (reserved >= 0 and reserved <= quantity),
    updated_at timestamptz not null default now()
);
