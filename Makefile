.PHONY: up down dev dev-down migrate seed build test logs health

SERVICES ?=
COMPOSE_DEV = docker compose -f docker-compose.yml -f docker-compose.dev.yml

up:
	docker compose up -d

down:
	docker compose down

dev:
	$(COMPOSE_DEV) up -d

dev-down:
	$(COMPOSE_DEV) down

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
