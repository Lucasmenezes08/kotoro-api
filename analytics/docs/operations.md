# Operating Kotoro Analytics: concurrency risks, protections and failure modes

This document lists what can go wrong when the Analytics API and the Analytics Consumer
run concurrently, how each risk is handled, and where it is tested. Decisions behind it:
[ADR 0001](adr/0001-read-database-decisions.md), [ADR 0002](adr/0002-summary-contract-and-identity.md),
[ADR 0003](adr/0003-chart-endpoints-contract.md).

## Components

```text
Go API → outbox_events → Outbox Relay → RabbitMQ → analytics-consumer → postgres-analytics ← analytics-api ← frontend
```

- **analytics-api** only reads, through sessions that PostgreSQL itself enforces as
  read-only (`default_transaction_read_only`).
- **analytics-consumer** is the only writer of the projections.
- **analytics-migrations** changes the schema; API and consumer wait for it.

## Concurrency risks and protections

| # | Risk | Protection | Tested in |
| - | ---- | ---------- | --------- |
| 1 | The relay publishes the same event twice (crash after the broker ACK, before `MarkPublished`): time counted twice | `processed_events` row inserted with `ON CONFLICT DO NOTHING` in **the same transaction** as the projections; a duplicate changes nothing | `test_projection_postgres.py::test_redelivered_event_is_applied_once`, `test_consumer_rabbitmq.py::test_redelivered_message_is_counted_once` |
| 2 | The same cycle arrives under a different event id | `study_cycle_facts` keyed by `cycle_id`; the second one is ignored | `test_same_cycle_under_another_event_id_is_counted_once` |
| 3 | Events arrive out of order (parallel relay workers, retries) | additive upserts; sprint first/last with `LEAST`/`GREATEST`; streak recomputed from the stored days | `test_out_of_order_events_give_the_same_result` |
| 4 | Two consumer replicas process events of the same user at the same time and recompute the streak from stale data | `pg_advisory_xact_lock` per user at the start of the transaction | `test_parallel_consumers_on_the_same_user_stay_consistent` (10 events, each delivered twice, all in parallel) |
| 5 | Crash between the database commit and the RabbitMQ ACK | message redelivered; risk 1's protection makes it harmless | covered by 1 |
| 6 | Crash before the commit | transaction rolled back, message not acked → redelivered; nothing half-written | atomic transaction per event |
| 7 | Consumer starts before the tables exist (Kubernetes has no ordering) and rejects the first events | Compose: `depends_on: service_completed_successfully`; Kubernetes: `wait_for_schema` initContainer | `test_wait_for_schema_postgres.py` |
| 8 | One `AsyncSession` shared by concurrent tasks | one session per HTTP request; services await their queries sequentially (ADR 0001) | design rule; `GetSummary` comment |
| 9 | One user reads another user's metrics | every query filters by the token's `user_id`; another user's sprint is a 404 identical to an unknown id | `test_*_endpoint_postgres.py`; **mutation-checked**: removing the filter makes 7 + 2 tests fail |
| 10 | The API writes to the read model by mistake | read-only PostgreSQL sessions | `test_api_session_cannot_write` |

## Failure modes

| Failure | API | Consumer | Recovery |
| ------- | --- | -------- | -------- |
| analytics database down | `/healthz` 200, `/readyz` 503, dashboard routes 503; Kubernetes stops routing to the pod (readiness) but does not restart it (liveness ignores the DB) | holds each message `ANALYTICS_CONSUMER_RETRY_DELAY_SECONDS`, then requeues it | automatic when the database returns |
| RabbitMQ down | unaffected (serves the last projection) | `connect_robust` reconnects and re-declares the topology | automatic; the Go relay still needs `docker compose restart outbox-relay` |
| Consumer down | unaffected, data becomes stale | — | events wait in the durable queue; projected as soon as it restarts (demonstrated) |
| Malformed or incompatible message | — | rejected without requeue, logged with the message id; the queue keeps flowing | **no dead-letter queue yet**: re-publish from the relay's outbox if needed |
| Bug while projecting one event | — | one retry, then reject + traceback in the log | fix and re-publish |
| Invalid / expired token | 401 | — | client logs in again |

## Consistency the dashboard can expect

Eventual: a finished cycle appears after the relay's next poll (up to
`OUTBOX_MAX_POLL_INTERVAL`, 5 s when idle) plus the consumer's transaction. Measured
end to end with the whole stack: **~2–4 s**. With the consumer stopped, the delay is the
downtime; nothing is lost.

## Observability

- JSON logs on stdout, one object per line; `correlation_id` is the HTTP `X-Request-ID`
  in the API and the event id in the consumer, so an event can be followed from the
  relay's log (`event_id=…`) to `event … projected`.
- Secrets, URL credentials and bearer tokens are redacted from every line, tracebacks
  included. Query strings and headers are never logged.

## Known limitations

- No dead-letter queue (same as the relay; a DLX should be designed with it).
- No metrics endpoint (Prometheus) or tracing yet.
- The streak is recomputed from the user's active days on every event: correct with any
  order, cost proportional to the number of active days.
- Per-user time zones are not supported (ADR 0001).
