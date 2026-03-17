# Hookman

> Reliable, self-hosted webhook delivery infrastructure — built with Go.

Hookman is a high-performance webhook delivery service that handles event ingestion, queuing, reliable delivery with retries, dead letter management, and observability.

## Quick Start

```bash
cp .env.example .env       # edit DATABASE_URL, API_KEY, SIGNING_SECRET
docker compose up
```

Dashboard: `http://localhost:4000/dashboard`
API: `http://localhost:4000/api`

## Development

```bash
# Prerequisites: Go 1.22+, Postgres 14+
make migrate
make dev
make test
```

See [docs/](docs/) for version history and decisions.
