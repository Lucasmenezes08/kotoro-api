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
| `GET /docs`    | Swagger UI (OpenAPI)                          |

The read database, `/readyz` and the dashboard endpoints come in the next stages.

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
