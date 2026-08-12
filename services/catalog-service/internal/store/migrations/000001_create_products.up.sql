CREATE TABLE products (
    id text primary key,
    name text not null,
    short_description text not null default '',
    description text not null default '',
    brand text not null,
    category_id text not null,
    category_name text not null,
    price_amount_minor bigint not null,
    currency text not null default 'VND',
    original_price_amount_minor bigint not null default 0,
    rating real not null default 0,
    review_count bigint not null default 0,
    sold_count bigint not null default 0,
    is_new boolean not null default false,
    badge text not null default '',
    featured boolean not null default false,
    emoji text not null default '',
    image_hue smallint[] not null default '{}',
    created_at timestamptz not null default now()
);

CREATE TABLE product_images (
    product_id text references products(id) on delete cascade,
    url text not null,
    position int not null,
    primary key (product_id, position)
);

CREATE TABLE product_variants (
    product_id text references products(id) on delete cascade,
    color text not null,
    color_hex text not null,
    position int not null,
    primary key (product_id, position)
);

CREATE TABLE product_details (
    product_id text references products(id) on delete cascade,
    detail text not null,
    position int not null,
    primary key (product_id, position)
);

-- Search support: exact category filter and case-insensitive name matching.
CREATE INDEX products_category_id_idx ON products (category_id);
CREATE INDEX products_name_lower_idx ON products (lower(name));
