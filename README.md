# OSShip

**[Live demo → https://osship.app](https://osship.app)**

## Project presentation

Video walkthrough of OSShip — what the platform does, how it works, and how the pieces fit together:

**[Watch on Tella → OSShip presentation](https://www.tella.tv/video/osship-presentation-5klw)**

**[Backup - Video presentation](https://livejaverianaedu-my.sharepoint.com/personal/sfelipe_galindor_javeriana_edu_co/_layouts/15/stream.aspx?id=%2Fpersonal%2Fsfelipe_galindor_javeriana_edu_co%2FDocuments%2FOSShip%20presentation%2Emp4&nav=eyJyZWZlcnJhbEluZm8iOnsicmVmZXJyYWxBcHAiOiJPbmVEcml2ZUZvckJ1c2luZXNzIiwicmVmZXJyYWxBcHBQbGF0Zm9ybSI6IldlYiIsInJlZmVycmFsTW9kZSI6InZpZXciLCJyZWZlcnJhbFZpZXciOiJNeUZpbGVzTGlua0NvcHkifX0&ga=1&referrer=StreamWebApp%2EWeb&referrerScenario=AddressBarCopied%2Eview%2E51b645dc-d51c-4888-81aa-4bec32cd99f4)**

## About

OSShip is an open-source mentorship platform that connects students with OSS maintainers through paid, structured mentorship on real projects. Students browse listings, enroll in multi-week slots, join live video sessions, and build verifiable portfolio evidence. Mentors publish listings with their own pricing and schedule; admins review mentor applications and oversee the public payout ledger.

The platform is built as a microservices monorepo orchestrated with Docker Compose:

| Layer | Stack |
|-------|-------|
| Frontend | Next.js UI behind Nginx |
| API | Go gateway with JWT auth, rate limiting, and Swagger docs |
| Services | Auth, users, listings, sessions, mentors, notifications (Go); payments and metrics (Rust) |
| Data | PostgreSQL, Redis, Kafka |
| Integrations | Stripe Connect (payments), Jitsi (live sessions), Resend (email), GitHub OAuth |
| Observability | Prometheus, Grafana, optional Sentry |

Application code lives in Git submodules; this repository provides orchestration, migrations, and infrastructure config.

## Run locally

### Prerequisites

| Tool | Purpose |
|------|---------|
| Git | Clone the meta-repo and its submodules |
| Docker Engine + Compose v2 | Run the full stack (`docker compose`) |
| Make | Project shortcuts in `Makefile` |
| Go | Migrations, seed data, Swagger generation, and tests |
| curl | Health-check script |

Node.js is only required if you run the UI outside Docker; the default stack builds it inside the `ui` image.

### 1. Clone

```bash
git clone --recurse-submodules git@github.com:OSShip/osship.git
cd osship
```

If you already cloned without submodules:

```bash
git submodule update --init --recursive
```

### 2. Configure environment

```bash
cp .env.example .env
```

Update `.env` only for integrations you want to exercise locally:

- `JWT_SECRET` — change for any non-throwaway environment.
- `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`, `GITHUB_TOKEN` — GitHub OAuth and profile workflows.
- `STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET` — real Stripe payment flows.
- `RESEND_API_KEY` — real email delivery.
- `SENTRY_DSN`, `NEXT_PUBLIC_SENTRY_DSN` — optional; empty values disable reporting.

### 3. Start the stack

```bash
make up
```

Nginx exposes the app on **port 3000** (mapped to port 80 inside the container).

### 4. Seed demo data (optional)

```bash
make seed
```

Demo accounts (password: `password123`):

| Role | Email |
|------|-------|
| Student | `student@osship.local` |
| Mentor | `mentor@osship.local` |
| Admin | `admin@osship.local` |

### 5. Verify

```bash
./scripts/health-check.sh http://localhost:3000
```

| URL | Description |
|-----|-------------|
| http://localhost:3000 | Web app |
| http://localhost:3000/api/v1/health | Gateway health |
| http://localhost:3000/api/docs/ | Swagger UI |
| http://localhost:3000/grafana/ | Grafana (`admin` / `admin`) |

### Development mode

To expose individual service ports on the host (UI on `:3000`, gateway on `:8080`, Postgres on `:5432`, etc.):

```bash
make dev
```

Common commands:

```bash
make logs          # Follow Docker Compose logs
make recreate      # Rebuild and recreate services
make dev-recreate  # Rebuild the development stack
make migrate       # Re-run database migrations
make down          # Stop the default stack
make dev-down      # Stop the development stack
```

If port `3000` is already in use, stop the conflicting process or change the `nginx` port mapping in `docker-compose.yml`.

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

## API documentation

Swagger UI is served by the gateway (generated with [swaggo/swag](https://github.com/swaggo/swag)):

| URL | Description |
|-----|-------------|
| http://localhost:3000/api/docs/ | Swagger UI |
| http://localhost:3000/api/docs/doc.json | OpenAPI spec (JSON) |

After changing endpoint annotations in `services/gateway/internal/apidoc/`:

```bash
make swagger
```

## Observability

| Layer | Tool | Local access |
|-------|------|--------------|
| Metrics | Prometheus + Grafana | http://localhost:3000/grafana/ |
| Errors | Sentry | Set `SENTRY_DSN` and `NEXT_PUBLIC_SENTRY_DSN` in `.env` |

## Production deploy

```bash
./scripts/deploy.sh
```

CI/CD is configured via Jenkins (`deploy/Jenkinsfile`).

## License

MIT — see [LICENSE](LICENSE).
