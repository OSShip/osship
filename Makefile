.PHONY: up down recreate dev dev-down dev-recreate migrate seed build test logs health swagger

SERVICES ?=
COMPOSE_DEV = docker compose -f docker-compose.yml -f docker-compose.dev.yml

up:
	docker compose up -d

down:
	docker compose down

recreate:
	docker compose up -d --force-recreate $(SERVICES)

dev:
	$(COMPOSE_DEV) up -d

dev-down:
	$(COMPOSE_DEV) down

dev-recreate:
	$(COMPOSE_DEV) up -d --force-recreate --build $(SERVICES)

migrate:
	./scripts/migrate.sh

seed:
	./scripts/seed.sh

build:
	docker buildx bake $(SERVICES)
test:
	go test ./utils/observability/...
	go test -tags=integration ./tests/integration/...

integration:
	./scripts/integration-test.sh

logs:
	docker compose logs -f

health:
	./scripts/health-check.sh

swagger:
	cd services/gateway && go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g main.go --parseInternal --output docs
