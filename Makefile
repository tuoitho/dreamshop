.PHONY: up down logs proto lint verify verify-identity verify-edge-auth identity-keys

up: identity-keys
	docker compose -f deployments/docker/compose.foundation.yml up -d --build
	docker compose -f deployments/docker/compose.foundation.yml up -d --force-recreate --no-deps kong

down:
	docker compose -f deployments/docker/compose.foundation.yml down

logs:
	docker compose -f deployments/docker/compose.foundation.yml logs -f

proto:
	buf generate

lint:
	buf lint

verify:
	pwsh -File scripts/verify-foundation.ps1

verify-identity:
	pwsh -NoProfile -File scripts/verify-identity.ps1

verify-edge-auth:
	pwsh -NoProfile -File scripts/verify-edge-auth.ps1

identity-keys:
	pwsh -NoProfile -File scripts/generate-identity-dev-keys.ps1
