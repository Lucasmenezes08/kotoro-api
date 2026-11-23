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

| `GET /api/v1/analytics/summary?from=&to=` | Dashboard summary of the authenticated user (requires a bearer token) |
| `GET /api/v1/analytics/daily?from=&to=` | One entry per day of the period (inactive days as zeros), ascending, for charts ([ADR 0003](docs/adr/0003-chart-endpoints-contract.md)) |
| `GET /api/v1/analytics/subjects?from=&to=` | Totals per subject with activity in the period, most studied first; ids only, names come from the Go API |
| `GET /api/v1/analytics/sprints/{sprint_id}` | Totals, first/last cycle (UTC) and per-subject split of one sprint; `404` when unknown, without finished cycles, or another user's |

```bash
# Token for the seed user Ana, signed with the local development secret
TOKEN=$(cd analytics && uv run python -c "from tests.support.tokens import make_token; print(make_token('11111111-1111-4111-8111-111111111111'))")
curl -H "Authorization: Bearer $TOKEN" "http://localhost:8001/api/v1/analytics/summary?from=2026-11-01&to=2026-11-30"
# {"period":{"from":"2026-11-01","to":"2026-11-30","timezone":"America/Sao_Paulo"},
#  "studied_seconds":75000,"completed_cycles":33,"completed_tasks":18,"active_days":16,
#  "streak":{"current_days":0,"longest_days":9,"last_active_date":"2026-11-20"}}
```

`streak.current_days` is relative to today: with the default seed (last active day
2026-11-20) it is `0` after 2026-11-21. Seed with `--end-date <today>` to see a live streak.
In the real flow, use the token returned by the Go API's `POST /auth/login`.

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

| `ANALYTICS_JWT_SECRET` | dev secret in `local`/`test`; **required** (>= 32 chars) in `production` | must equal the Go API's `JWT_SECRET` |
| `ANALYTICS_JWT_ISSUER` | `kotoro-api` | expected `iss` claim |

### Authentication

Dashboard endpoints identify the user **only** from the access token issued by the Go
API (`POST /auth/login`), sent as `Authorization: Bearer <token>`. The token must be
HS256, signed with the shared secret, not expired, with `iss = kotoro-api` and
`sub` = `user_id` = a UUID. Anything else returns
`401 {"error": "unauthorized"}` with `WWW-Authenticate: Bearer`. A user id in the URL,
query string or a custom header is never accepted. See
[ADR 0002](docs/adr/0002-summary-contract-and-identity.md).

### Errors

Every error body follows the Go API shape (`{"error": "..."}`):

| Status | When | Body |
| ------ | ---- | ---- |
| `401` | missing/invalid/expired token | `{"error": "unauthorized"}` |
| `422` | bad or missing parameter, `from` after `to`, period over 366 days | `{"error": "invalid request", "details": [{"field": "from", "message": "..."}]}` |
| `503` | analytics database down, unreachable or timed out | `{"error": "service unavailable"}` |
| `500` | anything unexpected (traceback only in the server log) | `{"error": "internal server error"}` |

Periods (`from`, `to`) are inclusive calendar days in `ANALYTICS_METRICS_TIMEZONE`.

Compose passes the same `JWT_SECRET` (local default
`kotoro-local-development-jwt-secret-0001`) to `api` and `analytics-api`.

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

## Development seed data

Deterministic data for local development and manual tests. Apply migrations first.

```bash
# inside Docker
docker compose run --rm analytics-api python -m kotoro_analytics.seeds

# locally
cd analytics
uv run python -m kotoro_analytics.seeds                       # last active day 2026-11-20
uv run python -m kotoro_analytics.seeds --end-date 2026-12-01 # shift the whole data set
```

Re-running replaces only the seed users' rows. The script refuses to run when
`ANALYTICS_ENVIRONMENT=production`. Aggregates are derived from the cycle facts, so all
tables agree with each other.

> These IDs exist only for local development. They are not real users and are not
> authentication: the identity mechanism is still being defined (see onboarding §8).

| User  | `user_id`                              | What it covers |
| ----- | -------------------------------------- | -------------- |
| Ana   | `11111111-1111-4111-8111-111111111111` | 16 active days from end-16 to end, gap at end-7 (longest streak 9, current 7); 33 cycles, 75 000 s, 18 tasks; 3 subjects; a 22:00 cycle that is next day in UTC but counts for the local day |
| Bruno | `22222222-2222-4222-8222-222222222222` | 3 days on the same dates as Ana (isolation between users); 3 cycles, 8 100 s, 6 tasks |
| Carla | `33333333-3333-4333-8333-333333333333` | no activity (empty dashboard, zeroed totals) |

| Subject (name for reference only) | `subject_id`                           |
| --------------------------------- | -------------------------------------- |
| Ana: Matemática                   | `aaaaaaaa-0000-4000-8000-000000000001` |
| Ana: Português                    | `aaaaaaaa-0000-4000-8000-000000000002` |
| Ana: Biologia                     | `aaaaaaaa-0000-4000-8000-000000000003` |
| Bruno: Física                     | `bbbbbbbb-0000-4000-8000-000000000001` |

Sprint, block, cycle and event IDs are derived with `uuid5` from a fixed namespace
(`kotoro_analytics.seeds.SEED_NAMESPACE`), so they are the same on every run. There is
one sprint per user and active day.

## Logs and request ids

Both processes log **one JSON object per line** to stdout
(`ANALYTICS_LOG_FORMAT=text` for a readable local terminal):

```json
{"timestamp": "2026-11-23T18:32:10.441500+00:00", "level": "INFO", "logger": "kotoro_analytics.access",
 "message": "GET /api/v1/analytics/summary 401", "correlation_id": "demo-req-43",
 "http_method": "GET", "http_path": "/api/v1/analytics/summary", "http_status": 401, "duration_ms": 7.68}
```

- **API:** every response carries `X-Request-ID`. A caller-provided id is reused when it
  matches `[A-Za-z0-9._-]{1,128}`; otherwise one is generated. It is the `correlation_id`
  of every log line of that request, including the `500` traceback.
- **Consumer:** `correlation_id` is the event `id`, so one event can be followed from the
  relay's log to the projection.
- **Access log:** one line per request from the middleware (method, path, status,
  duration). Query strings and headers are never logged; uvicorn's own access log is off.
- **Redaction** runs on the final line (messages and tracebacks): the JWT secret and the
  database/RabbitMQ passwords (when at least 12 characters), any `scheme://user:password@`
  credentials and `Bearer <token>` values become `***`. HTTP error bodies never contain
  exception details at all.

## Analytics Consumer

`python -m kotoro_analytics.consumer` (Compose service `analytics-consumer`) reads
`study_cycle.finished.v1` from the `kotoro.analytics.events` queue and updates every
projection. It never serves HTTP; the API never reads RabbitMQ.

```text
Go API -> outbox_events -> Outbox Relay -> RabbitMQ -> analytics-consumer -> postgres-analytics -> analytics-api
```

| Guarantee | How |
| --------- | --- |
| No double counting (redelivery) | `processed_events` insert (`ON CONFLICT DO NOTHING`) in the same transaction as the projections |
| No loss | message acked only after the transaction commits; a crash before that means redelivery |
| Out-of-order events | additive upserts, `LEAST`/`GREATEST` for sprint times, streak recomputed from stored days |
| Parallel consumers | per-user `pg_advisory_xact_lock` inside the transaction |
| Broker restart | `aio_pika.connect_robust` reconnects and re-declares the topology |
| Database down | message held for `ANALYTICS_CONSUMER_RETRY_DELAY_SECONDS`, then requeued |
| Malformed message | rejected without requeue and logged; the queue keeps flowing |

| Variable | Default |
| -------- | ------- |
| `ANALYTICS_RABBITMQ_URL` | `amqp://guest:guest@localhost:5672/` (Compose: `rabbitmq:5672`) |
| `ANALYTICS_RABBITMQ_EXCHANGE` | `kotoro.domain-events` |
| `ANALYTICS_RABBITMQ_QUEUE` | `kotoro.analytics.events` |
| `ANALYTICS_RABBITMQ_PREFETCH` | `10` |
| `ANALYTICS_CONSUMER_RETRY_DELAY_SECONDS` | `5` |

The consumer declares the same exchange, queue and binding as the relay
(`internal/messaging/rabbitmq/publisher.go`), so either can start first. Migrations run in
the one-shot `analytics-migrations` service before the API and the consumer start.

Known limitations:

- **No dead-letter queue**: rejected messages are only in the logs (same as the relay,
  which keeps failures in `outbox_events`).
- **Eventual consistency**: a finished cycle shows up after the relay polls (up to its
  `OUTBOX_MAX_POLL_INTERVAL`, 5 s when idle) plus consumer time; ~4 s in the demo below.
- The Go relay does not reconnect to RabbitMQ by itself; after a broker restart run
  `docker compose restart outbox-relay` (the consumer reconnects on its own).

### End-to-end demo

```bash
docker compose up --build -d
python analytics/scripts/demo_end_to_end.py
```

It registers a user in the Go API, finishes a real study cycle, waits until the event
reaches the Analytics API and prints `summary`, `daily`, `subjects` and the sprint.
To show fault tolerance, `docker compose stop analytics-consumer` before running it: the
event waits in the queue and is projected as soon as the consumer is started again.

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
uv run pytest                          # unit + API + integration
uv run pytest -m "not integration"     # no database needed
uv run pytest -m integration           # only PostgreSQL tests
uv run ruff check .
uv run ruff format --check .
```

| Folder               | What it checks                                          | Needs           |
| -------------------- | ------------------------------------------------------- | --------------- |
| `tests/unit`         | config, metric rules, streaks, seed data set            | nothing         |
| `tests/api`          | HTTP status and JSON (database check faked)             | nothing         |
| `tests/integration`  | migrations up/down and drift, real `/readyz`, seeds, read-only API session | PostgreSQL |

Integration tests use `ANALYTICS_TEST_DATABASE_URL`, by default
`postgresql+asyncpg://analytics:analytics@localhost:5433/kotoro_analytics_test`
(start it with `docker compose up -d postgres-analytics`). That database is **dropped
and recreated** on every run, and its name must end in `_test`, so the development
database `kotoro_analytics` is never touched. When PostgreSQL is unreachable the
integration tests are skipped, not failed.
