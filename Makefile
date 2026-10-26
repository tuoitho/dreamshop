.PHONY: infra-up infra-down infra-logs prod-config prod-up prod-down prod-logs identity order cart bff identity-keys proto lint verify check-boundaries up down help

PROD_ENV_FILE ?= .env.production

infra-up:
	docker compose -f deployments/docker/compose.foundation.yml up -d --build
	docker compose -f deployments/docker/compose.foundation.yml up -d --force-recreate --no-deps kong

infra-down:
	docker compose -f deployments/docker/compose.foundation.yml down

infra-logs:
	docker compose -f deployments/docker/compose.foundation.yml logs -f

prod-config:
	docker compose --env-file $(PROD_ENV_FILE) -f deployments/docker/compose.production.yml config --quiet

prod-up:
	docker compose --env-file $(PROD_ENV_FILE) -f deployments/docker/compose.production.yml up -d --build
	docker compose --env-file $(PROD_ENV_FILE) -f deployments/docker/compose.production.yml up -d --force-recreate --no-deps kong

prod-down:
	docker compose --env-file $(PROD_ENV_FILE) -f deployments/docker/compose.production.yml down

prod-logs:
	docker compose --env-file $(PROD_ENV_FILE) -f deployments/docker/compose.production.yml logs -f

identity:
	go run ./scripts/start-local -service identity-service

order:
	go run ./scripts/start-local -service order-service

cart:
	go run ./scripts/start-local -service cart-service

bff:
	go run ./scripts/start-local -service bff-service

identity-keys:
	go run ./scripts/generate-identity-dev-keys

proto:
	buf generate

lint:
	buf lint

verify:
ifdef SERVICE
	go run ./scripts/verify-service -service $(SERVICE)
else
	go run ./scripts/verify-foundation
endif

check-boundaries:
	go run ./scripts/check-boundaries

up: infra-up

down: infra-down

help:
	@echo "Targets:"
	@echo "  make infra-up          - Start PostgreSQL, Redis, RabbitMQ, and Kong"
	@echo "  make infra-down        - Stop local infrastructure"
	@echo "  make infra-logs        - Follow infrastructure logs"
	@echo "  make prod-config       - Validate production Docker Compose configuration"
	@echo "  make prod-up           - Build and start the full production Docker Compose stack"
	@echo "  make prod-down         - Stop the production Docker Compose stack"
	@echo "  make prod-logs         - Follow production Docker Compose logs"
	@echo "  make identity          - Start identity-service locally"
	@echo "  make order             - Start order-service locally"
	@echo "  make cart              - Start cart-service locally"
	@echo "  make bff               - Start bff-service locally"
	@echo "  make identity-keys     - Generate local Identity RSA keys"
	@echo "  make verify            - Check foundation files, Buf lint, and running Kong routes"
	@echo "  make verify SERVICE=<name> - Gate one surface: <name>-service, ecommerce-fe, or contracts"
	@echo "  make check-boundaries  - Fail on cross-service imports, dependencies, or database references"
