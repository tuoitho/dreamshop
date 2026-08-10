create table refresh_token_sessions (
    id uuid primary key,
    user_id uuid not null references identity_users(id) on delete cascade,
    family_id uuid not null,
    token_hash varchar(64) not null,
    issued_at timestamptz not null,
    expires_at timestamptz not null,
    revoked_at timestamptz,
    replaced_by_session_id uuid references refresh_token_sessions(id),
    revocation_reason varchar(32),
    constraint uq_refresh_token_sessions_token_hash unique (token_hash),
    constraint ck_refresh_token_sessions_revocation check (
      (revoked_at is null and revocation_reason is null)
      or (revoked_at is not null and revocation_reason in ('ROTATED','LOGOUT','REUSE_DETECTED'))
    )
);
create index ix_refresh_token_sessions_family_active
  on refresh_token_sessions (family_id) where revoked_at is null;
create index ix_refresh_token_sessions_user_expires
  on refresh_token_sessions (user_id, expires_at);
