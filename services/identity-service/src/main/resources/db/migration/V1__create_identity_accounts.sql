create table identity_users (
    id uuid primary key,
    email varchar(254) not null,
    normalized_email varchar(254) not null,
    password_hash varchar(255) not null,
    status varchar(32) not null,
    created_at timestamptz not null,
    updated_at timestamptz not null,
    constraint uq_identity_users_normalized_email unique (normalized_email),
    constraint ck_identity_users_status check (status in ('ACTIVE', 'DISABLED'))
);

create table identity_profiles (
    user_id uuid primary key references identity_users(id) on delete cascade,
    full_name varchar(100) not null
);

create table roles (
    id smallint primary key,
    code varchar(32) not null,
    constraint uq_roles_code unique (code)
);

create table user_roles (
    user_id uuid not null references identity_users(id) on delete cascade,
    role_id smallint not null references roles(id),
    primary key (user_id, role_id)
);

insert into roles (id, code) values (1, 'CUSTOMER');
