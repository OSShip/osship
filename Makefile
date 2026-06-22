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
	./scripts/health-check.sh

logs:
	docker compose logs -f

health:
	./scripts/health-check.sh
