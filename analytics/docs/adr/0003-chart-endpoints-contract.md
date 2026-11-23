# ADR 0003: Contracts of the chart endpoints (daily, subjects, sprint)

- Status: **proposed** (to be confirmed with the frontend)
- Date: 2026-11-23
- Scope: stage 4 endpoints

## Context

Stage 4 adds the queries behind the dashboard charts. They reuse everything fixed in
ADR 0002: identity from the bearer token, `from`/`to` inclusive periods in
`ANALYTICS_METRICS_TIMEZONE` (max 366 days), error bodies `{"error": ...}` and `422` for
any invalid input.

Facts that shape these contracts:

- The event carries ids only: **subject names and sprint names/status are not in the
  analytics database**. The frontend already has them from the Go API.
- `user_daily_metrics` has one row per *active* day; inactive days have no row.
- `user_subject_metrics` has one row per user, day and subject.
- `sprint_summaries` has totals per sprint; per-subject detail of a sprint comes from
  `study_cycle_facts` (indexed by `sprint_id`).

Examples use the development seed (user Ana, `analytics/README.md`).

## Decision 1: daily series

```http
GET /api/v1/analytics/daily?from=2026-11-12&to=2026-11-14
```

```json
{
  "period": { "from": "2026-11-12", "to": "2026-11-14", "timezone": "America/Sao_Paulo" },
  "days": [
    { "date": "2026-11-12", "studied_seconds": 4500, "completed_cycles": 2, "completed_tasks": 1 },
    { "date": "2026-11-13", "studied_seconds": 0, "completed_cycles": 0, "completed_tasks": 0 },
    { "date": "2026-11-14", "studied_seconds": 4500, "completed_cycles": 2, "completed_tasks": 1 }
  ]
}
```

- **One entry per calendar day of the period**, inactive days included with zeros, so a
  chart can plot the array as is. `len(days)` equals the number of days in the period.
- Ordered by `date` ascending.
- No pagination: the period limit (366 days) bounds the size.

## Decision 2: subjects

```http
GET /api/v1/analytics/subjects?from=2026-11-01&to=2026-11-30
```

```json
{
  "period": { "from": "2026-11-01", "to": "2026-11-30", "timezone": "America/Sao_Paulo" },
  "subjects": [
    { "subject_id": "aaaaaaaa-0000-4000-8000-000000000001", "studied_seconds": 27000, "completed_cycles": 12, "completed_tasks": 8 },
    { "subject_id": "aaaaaaaa-0000-4000-8000-000000000002", "studied_seconds": 25500, "completed_cycles": 11, "completed_tasks": 5 },
    { "subject_id": "aaaaaaaa-0000-4000-8000-000000000003", "studied_seconds": 22500, "completed_cycles": 10, "completed_tasks": 5 }
  ]
}
```

- Only subjects **with activity in the period**; the API does not know the user's full
  subject list. An empty period returns `"subjects": []`.
- Ordered by `studied_seconds` descending, then `subject_id` ascending (deterministic).
- No names: the frontend joins `subject_id` with the subjects it loads from the Go API.
- No percentages: the frontend derives them from the values (avoids rounding rules here).
- No pagination: a student has a handful of subjects.

## Decision 3: sprint summary

```http
GET /api/v1/analytics/sprints/{sprint_id}
```

```json
{
  "sprint_id": "34603048-a584-5fe3-b49c-7f792b0fe040",
  "studied_seconds": 7500,
  "completed_cycles": 3,
  "completed_tasks": 3,
  "first_cycle_started_at": "2026-11-20T12:00:00Z",
  "last_cycle_ended_at": "2026-11-21T01:50:00Z",
  "subjects": [
    { "subject_id": "aaaaaaaa-0000-4000-8000-000000000001", "studied_seconds": 4500, "completed_cycles": 2, "completed_tasks": 3 },
    { "subject_id": "aaaaaaaa-0000-4000-8000-000000000002", "studied_seconds": 3000, "completed_cycles": 1, "completed_tasks": 0 }
  ]
}
```

- No period: a sprint is already bounded.
- Timestamps in UTC (ISO 8601 with `Z`); the frontend formats them locally.
- `subjects` uses the same item shape and ordering as decision 2.
- **`404 {"error": "not found"}`** when the caller has no analytics for that sprint:
  it does not exist, has no finished cycle yet, **or belongs to another user**. These
  cases are indistinguishable on purpose, so the endpoint never reveals that another
  user's sprint exists. The frontend should show "no data yet" for 404.
- A malformed `sprint_id` is `422`, like any other invalid input.

## Consequences

- Every query filters by the token's `user_id`, including the sprint lookup (the sprint
  id alone is never enough).
- Chart endpoints share `period` and the subject item shape with the summary, so the
  frontend can reuse types.
- Adding names later requires the events (or a new projection) to carry them; that is a
  contract change, not a query change.
