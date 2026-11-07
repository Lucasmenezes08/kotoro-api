# `study_cycle.finished.v1`

## Description

Records the successful completion of a study cycle. It gives Analytics the
cycle, subject, timing, execution mode, stop reason, and completed tasks
without requiring a read from the transactional database.

- Producer: `StudyCycleService.Stop`
- Aggregate: `study_cycle` (`aggregate_id` is `cycle_id`)
- Contract version: `1`
- Future routing key: `study_cycle.finished.v1`

The producer creates the event only after the cycle, its selected tasks, block
tasks, and sprint block updates have succeeded, and persists it in the same
database transaction. It is not created for cancelled cycles; those will use a
future `study_cycle.cancelled.v1` contract.

## Envelope

| Field | Type | Meaning |
| --- | --- | --- |
| `id` | UUID | Unique event identifier. |
| `user_id` | UUID | Owner of the sprint and cycle. |
| `aggregate_type` | string | Always `study_cycle`. |
| `aggregate_id` | UUID | Study cycle identifier. |
| `event_type` | string | Always `study_cycle.finished`. |
| `event_version` | integer | Always `1`; this is the contract version. |
| `occurred_at` | RFC 3339 timestamp | Service clock time at completion. |
| `payload` | JSON object | Versioned domain data below. |

`correlation_id` and `causation_id` are currently null.

## Payload

| Field | Type |
| --- | --- |
| `cycle_id` | UUID |
| `sprint_id` | UUID |
| `sprint_block_id` | UUID |
| `subject_id` | UUID |
| `sequence_number` | integer |
| `execution_mode` | string |
| `timer_type` | string |
| `started_at` | RFC 3339 timestamp |
| `ended_at` | RFC 3339 timestamp |
| `elapsed_seconds` | integer |
| `stop_reason` | string |
| `completed_task_ids` | UUID array (empty when no tasks were completed) |

Example:

```json
{
  "cycle_id": "a95642cc-1d65-4ac6-860b-1247c2e4dc20",
  "sprint_id": "e9fac54e-6be1-4c02-b6bd-fb8ad9d9a1b7",
  "sprint_block_id": "9542f658-2a58-4923-9690-9c02b6a7c821",
  "subject_id": "c1d4d6e8-0123-4cda-a4fd-7ce317b6c159",
  "sequence_number": 2,
  "execution_mode": "tarefa_individual",
  "timer_type": "temporizado",
  "started_at": "2026-11-04T12:00:00Z",
  "ended_at": "2026-11-04T12:25:00Z",
  "elapsed_seconds": 1500,
  "stop_reason": "concluido",
  "completed_task_ids": ["b8dcc5e9-598e-4628-84b5-5a6c5c4e12a1"]
}
```

## Delivery and consumers

An outbox relay and message broker are deliberately outside this change. When
delivery is introduced, consumers should expect at-least-once delivery and be
idempotent using the envelope `id`. Expected consumers include Analytics and
future user activity or reporting projections.
