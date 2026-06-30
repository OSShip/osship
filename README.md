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

## Dependencies

Install these tools before running the project locally:

| Dependency | Purpose |
|------------|---------|
| Git | Clone the meta-repo and its submodules |
| Docker Engine | Run Postgres, Redis, Kafka, backend services, UI, Nginx, Prometheus, and Grafana |
| Docker Compose v2 | Orchestrate the local stack through `docker compose` |
| Make | Run the project shortcuts in `Makefile` |
| Go | Run local helper commands, tests, seed data, and Swagger generation |
| curl | Run the health-check script |

Node.js and package-manager dependencies are handled inside the UI Docker image for the normal local stack.

## Local setup

1. Copy the example environment file:

   ```bash
   cp .env.example .env
   ```

2. Update `.env` only for integrations you want to exercise locally:

   - `JWT_SECRET` should be changed for any non-throwaway environment.
   - `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`, and `GITHUB_TOKEN` are only needed for GitHub OAuth/profile workflows.
   - `STRIPE_SECRET_KEY` and `STRIPE_WEBHOOK_SECRET` are only needed for real Stripe payment flows.
   - `RESEND_API_KEY` is only needed for real email delivery.
   - `SENTRY_DSN` and `NEXT_PUBLIC_SENTRY_DSN` are optional; empty values disable reporting.

3. Start the full stack:

```bash
make up
```

4. Seed demo data:

```bash
make seed
```

The seed script creates demo users with the password `password123`:

| Role | Email |
|------|-------|
| Student | `student@osship.local` |
| Mentor | `mentor@osship.local` |
| Admin | `admin@osship.local` |

5. Verify the stack:

```bash
make health
```

## Running locally

Use the default stack when you want the app available through Nginx:

```bash
make up
```

| URL | Description |
|-----|-------------|
| http://localhost | Web app |
| http://localhost/api/v1/health | Gateway health endpoint |
| http://localhost/api/docs/ | Swagger UI |
| http://localhost/grafana/ | Grafana (`admin` / `admin` by default) |

Use the development stack when you also want direct host access to the individual services:

```bash
make dev
```

| Host port | Service |
|-----------|---------|
| `3000` | UI |
| `5432` | Postgres |
| `8080` | Gateway |
| `8081` | Auth |
| `8082` | Listings |
| `8083` | Users |
| `8084` | Sessions |
| `8085` | Mentors |
| `8086` | Notifications |
| `8087` | Payments |
| `8088` | Metrics |
| `9090` | Prometheus |
| `3030` | Grafana |

Common commands:

```bash
make logs          # Follow Docker Compose logs
make recreate      # Recreate running services
make dev-recreate  # Rebuild and recreate the development stack
make migrate       # Re-run migrations against the running Postgres container
make down          # Stop the default stack
make dev-down      # Stop the development stack
```

If port `80` is already in use, stop the conflicting process or change the `nginx` port mapping in `docker-compose.yml` before running `make up`.

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
