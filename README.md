# Cookie's BnB

Booking and operations platform for accommodation businesses, property owners,
agents, staff, and guests.

## Project status

This repository is at the foundation stage. The architecture, relational data
model, and delivery plan are documented; the first PostgreSQL migration
establishes tenant-aware identity, property inventory, reservation integrity,
payments, and audit events. External payment, messaging, storage, and mapping
integrations must remain disabled until their server-side credentials and
provider verification are configured.

## Documentation

- [Architecture and project structure](docs/architecture.md)
- [Database design and migration notes](docs/database.md)
- [API reference](docs/api.md)
- [Implementation roadmap](docs/roadmap.md)

## Planned stack

- Web: Next.js, React, TypeScript, Tailwind CSS.
- API: Go, Gin, REST, OpenAPI.
- Persistence: PostgreSQL with versioned SQL migrations.
- Integrations: provider adapters for M-Pesa, email/SMS, object storage, maps,
  and WhatsApp.

See the architecture and database documents for tenancy, booking consistency,
payment trust boundaries, and delivery assumptions.

## Local API foundation

Requirements: Go 1.24+ and Docker Compose.

1. Copy `.env.example` to `.env` and set a local `POSTGRES_PASSWORD` matching
    the password in `DATABASE_URL`.
2. Run `docker compose up -d db` from the repository root. On a fresh database,
    Compose applies the foundation migration automatically.
3. Export the API settings with `set -a; . ./.env; set +a` and run
    `cd services/api && go run ./cmd/api`.
4. Check `http://localhost:8080/api/v1/healthz` and
    `http://localhost:8080/api/v1/readyz`.
5. Run API tests with `cd services/api && go test ./...`.

Authentication endpoints and request examples are in [API reference](docs/api.md).
For deployment, set `APP_ORIGIN` to the exact browser origin and
`COOKIE_SECURE=true` behind HTTPS. The example uses local-only development
settings.

Compose initialization scripts run only when the PostgreSQL data volume is
empty. Apply later schema changes with a versioned migration workflow; do not
delete a volume containing data as a migration strategy.