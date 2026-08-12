.PHONY: infra-up infra-down infra-logs identity order identity-keys identity-keys-linux proto lint verify verify-identity verify-edge-auth up down help identity-linux order-linux

infra-up:
	docker compose -f deployments/docker/compose.foundation.yml up -d --build
	docker compose -f deployments/docker/compose.foundation.yml up -d --force-recreate --no-deps kong

infra-down:
	docker compose -f deployments/docker/compose.foundation.yml down

infra-logs:
	docker compose -f deployments/docker/compose.foundation.yml logs -f

identity:
	pwsh -NoProfile -File services/identity-service/start-local.ps1

identity-linux:
	bash services/identity-service/start-local.sh

order:
	pwsh -NoProfile -File services/order-service/start-local.ps1

order-linux:
	bash services/order-service/start-local.sh

identity-keys:
	pwsh -NoProfile -File scripts/generate-identity-dev-keys.ps1

identity-keys-linux:
	bash scripts/generate-identity-dev-keys.sh

proto:
	buf generate

lint:
	buf lint

verify:
	pwsh -NoProfile -File scripts/verify-foundation.ps1

verify-identity:
	pwsh -NoProfile -File scripts/verify-identity.ps1

verify-edge-auth:
	pwsh -NoProfile -File scripts/verify-edge-auth.ps1

up: infra-up

down: infra-down

help:
	@echo "Targets:"
	@echo "  make infra-up          - Start PostgreSQL, Redis, RabbitMQ, and Kong"
	@echo "  make infra-down        - Stop local infrastructure"
	@echo "  make infra-logs        - Follow infrastructure logs"
	@echo "  make identity          - Start identity-service on Windows"
	@echo "  make identity-linux    - Start identity-service on Linux/WSL"
	@echo "  make order             - Start order-service on Windows"
	@echo "  make order-linux       - Start order-service on Linux/WSL"
	@echo "  make identity-keys     - Generate local Identity RSA keys on Windows"
	@echo "  make identity-keys-linux - Generate local Identity RSA keys on Linux/WSL"
	@echo "  make verify-identity   - Verify register, login, and JWKS"
	@echo "  make verify-edge-auth  - Verify Kong refresh and profile flow"
