# OSShip

Meta-repository for the OSShip open-source mentorship platform. Application code lives in Git submodules; this repo provides orchestration, migrations, and infrastructure config.

## Submodules

| Path | Repository | Description |
|------|------------|-------------|
| `ui/` | [OSShip/ui](https://github.com/OSShip/ui) | Next.js frontend |
| `utils/` | [OSShip/utils](https://github.com/OSShip/utils) | Shared Go libraries |
| `services/auth/` | [OSShip/auth](https://github.com/OSShip/auth) | Authentication |
| `services/gateway/` | [OSShip/gateway](https://github.com/OSShip/gateway) | API gateway |
| `services/listings/` | [OSShip/listings](https://github.com/OSShip/listings) | Mentorship listings |
| `services/users/` | [OSShip/users](https://github.com/OSShip/users) | User profiles |
| `services/sessions/` | [OSShip/sessions](https://github.com/OSShip/sessions) | Mentorship sessions |
| `services/mentors/` | [OSShip/mentors](https://github.com/OSShip/mentors) | Mentor applications |
| `services/notifications/` | [OSShip/notifications](https://github.com/OSShip/notifications) | Email notifications |
| `services/payments/` | [OSShip/payments](https://github.com/OSShip/payments) | Stripe payments |
| `services/metrics/` | [OSShip/metrics](https://github.com/OSShip/metrics) | Event metrics |

## Clone

```bash
git clone --recurse-submodules git@github.com:OSShip/osship.git
cd osship
```

If you already cloned without submodules:

```bash
git submodule update --init --recursive
```

## Quick start

```bash
cp .env.example .env
make up
make seed
```

## API documentation

Swagger UI is served by the gateway (auto-generated with [swaggo/swag](https://github.com/swaggo/swag)):

| URL | Description |
|-----|-------------|
| http://localhost/api/docs/ | Swagger UI |
| http://localhost/api/docs/doc.json | OpenAPI spec (JSON) |

After changing endpoint annotations in `services/gateway/internal/apidoc/`, regenerate:

```bash
make swagger
```

## Observability

| Layer | Tool | Access |
|-------|------|--------|
| Metrics | Prometheus + Grafana | http://localhost/grafana/ (admin / admin) |
| Errors | Sentry | Set `SENTRY_DSN` and `NEXT_PUBLIC_SENTRY_DSN` in `.env` |

When Sentry DSN vars are empty, all services run normally without reporting.

### Production deploy

```bash
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d
./scripts/deploy.sh
```

## License

MIT — see [LICENSE](LICENSE).
