# ADR 0001: Read database decisions

- Status: accepted
- Date: 2026-11-22
- Scope: `analytics/` (Analytics API and, later, Analytics Consumer)

## Context

Stage 2 of the Analytics onboarding connects the FastAPI service to a dedicated
PostgreSQL read database. Before writing models, migrations and queries, three
choices need to be fixed because they shape every table and every query.

## Decision 1: async SQLAlchemy end to end

We use **SQLAlchemy 2 in async mode** with the **asyncpg** driver.

Rules that follow from this choice:

- Use `AsyncEngine` and `AsyncSession` everywhere; do not mix in sync sessions.
- One `AsyncSession` per HTTP request, provided by a FastAPI dependency and closed
  when the request ends.
- Never share an `AsyncSession` between concurrent tasks (`asyncio.gather`,
  background tasks). Each task opens its own session.
- Routes that touch the database are `async def`.
- Database tests run with an async test runner against a real PostgreSQL.
- Alembic runs migrations through the async engine (`async` env template).

## Decision 2: the read model follows the published event contract

Tables and the future consumer follow
[`docs/events/study_cycle.finished.v1.md`](../../../docs/events/study_cycle.finished.v1.md),
which is what the Go API already writes to the outbox and the relay publishes to
RabbitMQ. The example envelope in the onboarding document is not used.

| Onboarding draft        | Published contract (used)  |
| ----------------------- | -------------------------- |
| `event_id`              | `id`                       |
| `data`                  | `payload`                  |
| `credited_seconds`      | `elapsed_seconds`          |
| `stop_reason: completed`| `stop_reason: concluido` / `tempo_esgotado` |

Column names in the read model match the payload field names
(`elapsed_seconds`, `stop_reason`, `execution_mode`, `timer_type`, ...).
Idempotency uses the envelope `id` as the primary key of `processed_events`.

Any incompatible change to the event requires a new version
(`study_cycle.finished.v2`), not a silent change to v1.

## Decision 3: metric day in `America/Sao_Paulo`

Daily aggregates (`user_daily_metrics.metric_date`, streaks, "studied today")
use the calendar day in **`America/Sao_Paulo`**, derived from the cycle's
`ended_at`. Event timestamps stay in UTC (`TIMESTAMPTZ`); only the day bucket is
converted.

The zone is a single setting (`ANALYTICS_METRICS_TIMEZONE`), not hard-coded in
queries.

## Consequences

- A cycle that ends at 22:00 in Brasília counts for that day, not the next UTC day.
- Per-user time zones are out of scope. Supporting them later requires the event
  (or a user projection) to carry the user's zone and a rebuild of the daily
  projections.
- Async adds test complexity (async fixtures, event loop handling); this is
  accepted to keep one consistent model across the API and the consumer.
- If the Go event contract changes, the read model changes through a new event
  version and a new migration, never by editing v1 behaviour.
