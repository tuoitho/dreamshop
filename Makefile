.PHONY: up down logs proto lint verify

up:
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
