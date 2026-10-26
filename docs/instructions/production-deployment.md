# Production Docker Compose Deployment

`deployments/docker/compose.production.yml` builds and runs the complete single-host container stack (`postgres`, `redis`, `rabbitmq`, `kong`, `identity-service`, `bff-service`, `catalog-service`, `inventory-service`, `cart-service`, `order-service`, and `ecommerce-fe`) on an isolated internal network (`ecommerce-prod-net`).

Kong is the sole container publishing a host port (`${PUBLIC_HTTP_PORT:-80}:8000`), serving both `/api/v1/*` and the `ecommerce-fe` SPA (`/`). Local hybrid development (`deployments/docker/compose.foundation.yml`) remains unchanged.

## 1. Provision RSA Signing Keys

Generate a dedicated production RSA keypair outside the repository (do **not** reuse `deployments/secrets/identity/` development keys) and grant read access to `appuser` (`UID 10001`) inside `identity-service`:

```bash
sudo mkdir -p /etc/ecommerce/secrets/identity
sudo chmod 0755 /etc/ecommerce /etc/ecommerce/secrets /etc/ecommerce/secrets/identity
sudo openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out /etc/ecommerce/secrets/identity/private.pem
sudo openssl rsa -pubout -in /etc/ecommerce/secrets/identity/private.pem -out /etc/ecommerce/secrets/identity/public.pem
sudo chown 10001:10001 /etc/ecommerce/secrets/identity/private.pem
sudo chmod 0400 /etc/ecommerce/secrets/identity/private.pem
sudo chmod 0644 /etc/ecommerce/secrets/identity/public.pem
```

## 2. Configure `.env.production`

Copy `.env.production.example` to `.env.production` (git-ignored) and set required secrets:

```bash
cp .env.production.example .env.production
```

Required settings:

- `POSTGRES_PASSWORD`: strong URL-safe password (`[A-Za-z0-9_-]`, for example `openssl rand -hex 24`) because Go services interpolate it into `DATABASE_URL` URIs.
- `RABBITMQ_DEFAULT_PASS`: strong broker password (`openssl rand -hex 24`).
- `IDENTITY_KEYS_DIR`: absolute path to the directory containing `private.pem` and `public.pem` (`/etc/ecommerce/secrets/identity`).
- `IDENTITY_SESSION_COOKIE_SECURE`: keep `true` when traffic reaches users over HTTPS (through an upstream reverse proxy/CDN such as Nginx, Caddy, or Cloudflare forwarding `Host`, `X-Forwarded-Proto: https`, and `X-Forwarded-For`); set `false` only for plain-HTTP staging verification.
- `IDENTITY_BOOTSTRAP_SUPER_ADMIN_*`: leave all three blank unless bootstrapping an initial `SUPER_ADMIN` account (password must be 12-128 characters).

Validate the rendered Compose configuration before starting containers:

```bash
make prod-config
# Equivalent:
docker compose --env-file .env.production -f deployments/docker/compose.production.yml config --quiet
```

## 3. Build and Start the Stack

Ensure generated protobuf artifacts in `contracts/go` and `contracts/java` are up to date (tracked in Git), then build and launch in detached mode:

```bash
make prod-up
# Equivalent:
docker compose --env-file .env.production -f deployments/docker/compose.production.yml up -d --build
```

Startup follows `service_healthy` dependencies automatically:

1. `postgres`, `redis`, `rabbitmq`
2. `identity-service` (Flyway migrations), `inventory-service` (migrations + seed), `order-service`, `ecommerce-fe`
3. `catalog-service` (migrations + seed + gRPC client to Identity authorization)
4. `cart-service` (migrations + gRPC clients to Catalog, Inventory, and Identity authorization)
5. `bff-service`
6. `kong`

## 4. Verify Health and Public Routes

Internal Actuator management ports (`8090` on `identity-service`, `8093` on `order-service`) and backend Go `/healthz` endpoints are restricted to `ecommerce-prod-net` and are not exposed publicly through Kong. Verify container health and public ingress routes:

```bash
docker compose --env-file .env.production -f deployments/docker/compose.production.yml ps
curl -fsS http://localhost:${PUBLIC_HTTP_PORT:-80}/ > /dev/null
curl -fsS http://localhost:${PUBLIC_HTTP_PORT:-80}/api/v1/storefront/products > /dev/null
curl -fsS http://localhost:${PUBLIC_HTTP_PORT:-80}/api/v1/auth/.well-known/jwks.json > /dev/null
```

Follow container logs when diagnosing issues:

```bash
make prod-logs
```

## 5. Rolling Out Updates and Stopping Safely

Rebuild and recreate only changed services without downtime to stateful volumes:

```bash
docker compose --env-file .env.production -f deployments/docker/compose.production.yml up -d --build <service-name>
```

Stop containers while preserving `postgres_data`, `redis_data`, and `rabbitmq_data` named volumes:

```bash
make prod-down
# Equivalent:
docker compose --env-file .env.production -f deployments/docker/compose.production.yml down
```

Never pass `-v` (`--volumes`) to `down` in production unless intentionally destroying persisted database and broker state.
