.PHONY: up down migrate seed build test logs health

up:
	docker compose up -d

down:
	docker compose down

migrate:
	./scripts/migrate.sh

seed:
	./scripts/seed.sh

build:
	docker compose build
test:
	go test ./packages/observability/...
	go test -tags=integration ./tests/integration/...

integration:
	./scripts/integration-test.sh

logs:
	docker compose logs -f

health:
	./scripts/health-check.sh
