# Kotoro Analytics API

Read-only HTTP API (FastAPI) that serves the Kotoro performance dashboard.

It is the **query side** of the event-driven architecture:

```text
Kotoro API (Go) -> outbox_events -> Outbox Relay -> RabbitMQ -> Analytics Consumer -> PostgreSQL Analytics -> Analytics API
```

The Analytics API only reads the analytics database. It never queries the transactional
database of the Go API and never consumes RabbitMQ directly.

## Current scope (stage 1: bootstrap)

| Endpoint       | Description                                   |
| -------------- | --------------------------------------------- |
| `GET /healthz` | Liveness check. Does not depend on a database |
| `GET /readyz`  | Readiness check. `200` when the analytics database answers `SELECT 1` within `ANALYTICS_DB_READINESS_TIMEOUT_SECONDS` (default `2`), `503` otherwise |
| `GET /docs`    | Swagger UI (OpenAPI)                          |

`/healthz` stays `200` while the database is down; only `/readyz` reports it.
Failures are logged with the exception type only, never the message.

When the `postgres-analytics` container is stopped, the log may also show
`Future exception was never retrieved ... gaierror`. It comes from asyncpg after the
readiness timeout cancels a pending DNS lookup and is harmless.

The dashboard endpoints come in the next stages.

## Layout

```text
analytics/
├── src/kotoro_analytics/
│   ├── api/
│   │   ├── main.py          # application factory (create_app)
│   │   └── routers/
│   │       └── health.py
│   └── config.py            # settings from ANALYTICS_* env vars
├── tests/
│   └── api/
├── Dockerfile
└── pyproject.toml
```

## Configuration

| Variable                | Default            | Allowed values                    |
| ----------------------- | ------------------ | --------------------------------- |
| `ANALYTICS_APP_NAME`    | `kotoro-analytics` | any string                        |
| `ANALYTICS_ENVIRONMENT` | `local`            | `local`, `test`, `production`     |
| `ANALYTICS_LOG_LEVEL`   | `info`             | `debug`, `info`, `warning`, `error` |
| `ANALYTICS_DATABASE_URL` | `postgresql+asyncpg://analytics:analytics@localhost:5433/kotoro_analytics` | async SQLAlchemy URL of the read database |
| `ANALYTICS_METRICS_TIMEZONE` | `America/Sao_Paulo` | IANA zone used to bucket daily metrics (see [ADR 0001](docs/adr/0001-read-database-decisions.md)) |

The read database runs as the `postgres-analytics` Compose service, exposed on host
port `5433` so it does not clash with the main `postgres` on `5432`. Its credentials
come from `ANALYTICS_POSTGRES_DB`, `ANALYTICS_POSTGRES_USER` and
`ANALYTICS_POSTGRES_PASSWORD` (defaults: `kotoro_analytics` / `analytics` / `analytics`).

## Read model and migrations

Tables are projections of `study_cycle.finished.v1`, defined in
`src/kotoro_analytics/infrastructure/database/models.py` and created by Alembic
migrations in `migrations/versions/`.

| Table                  | Grain                       | Purpose                                  |
| ---------------------- | --------------------------- | ---------------------------------------- |
| `processed_events`     | event `id`                  | idempotency (inbox) for the consumer     |
| `study_cycle_facts`    | finished cycle              | raw facts for drill-down and rebuilds    |
| `user_daily_metrics`   | user + day                  | summary and daily series                 |
| `user_subject_metrics` | user + day + subject        | per-subject performance by period        |
| `sprint_summaries`     | sprint                      | sprint summary                           |
| `user_streaks`         | user                        | current and longest streak               |

Days (`metric_date`) are calendar days in `ANALYTICS_METRICS_TIMEZONE`.

Apply migrations (the API itself never runs them):

```bash
# inside Docker
docker compose run --rm analytics-api alembic upgrade head

# locally, against postgres-analytics on localhost:5433
cd analytics
uv run alembic upgrade head
uv run alembic downgrade -1   # revert the last migration
uv run alembic check          # fails if models and migrations drifted
```

New migration after changing the models:

```bash
uv run alembic revision --autogenerate --rev-id 0002 -m "describe change"
```

Review the generated file before committing it.

## Running with Docker Compose

From the repository root:

```bash
docker compose up --build analytics-api
curl http://localhost:8001/healthz
# {"status":"ok"}
```

## Running locally

Requires Python 3.12 and [uv](https://docs.astral.sh/uv/).

```bash
cd analytics
uv sync
uv run uvicorn kotoro_analytics.api.main:app --reload --port 8001
```

## Tests and lint

```bash
cd analytics
uv run pytest
uv run ruff check .
uv run ruff format --check .
```
