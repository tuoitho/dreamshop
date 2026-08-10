.PHONY: identity order help

identity:
	set "IDENTITY_JWT_PRIVATE_KEY_PATH=$(CURDIR)/deployments/secrets/identity/private.pem" && set "IDENTITY_JWT_PUBLIC_KEY_PATH=$(CURDIR)/deployments/secrets/identity/public.pem" && mvn -f services/identity-service/pom.xml spring-boot:run

order:
	mvn -f services/order-service/pom.xml spring-boot:run

help:
	@echo "Targets:"
	@echo "  make identity   - Start identity-service"
	@echo "  make order      - Start order-service"
