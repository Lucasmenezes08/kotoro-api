# ADR 0002: Summary contract, user identity and error convention

- Status: **proposed** (to be confirmed with the Go API owner and the frontend)
- Date: 2026-11-23
- Scope: `GET /api/v1/analytics/summary` and every dashboard endpoint after it

## Context

Stage 3 delivers the first dashboard endpoint. Three things must be fixed before it,
because every later endpoint reuses them:

1. how the API knows **who** is asking;
2. the **shape** of the summary response and the meaning of the period;
3. how **errors** look (status codes and body).

Facts found in the codebase when this was written:

- The Go API already issues JWTs (`internal/auth/token.go`): HS256, `sub` = user id
  (UUID), `iss` = `kotoro-api`, 24h expiry, secret from `JWT_SECRET`.
- When `JWT_SECRET` is empty the Go API falls back to a hard-coded default secret.
- Go error bodies are `{"error": "<message>"}` (e.g. `401 {"error": "Unauthorized"}`).
- The Go API also serves `GET /analytics/subjects` and `GET /analytics/streak` with
  `timeframe` and `reference_date` query parameters, reading the transactional database.

## Decision 1: identity comes from the Go API's JWT

The Analytics API validates the same bearer token the frontend already sends to the
Go API. No user id is accepted from the URL, query string or a custom header.

- Header: `Authorization: Bearer <token>`.
- Algorithm pinned to **HS256**; any other `alg` (including `none`) is rejected.
- Required claims: `sub` (must be a UUID), `exp`, `iss` = `kotoro-api`.
  `nbf` and `exp` are enforced.
- The secret comes from `ANALYTICS_JWT_SECRET`, which must hold the same value as the
  Go API's `JWT_SECRET`.
- **No default secret.** The service refuses to start without it, except when
  `ANALYTICS_ENVIRONMENT` is `local` or `test`, where a documented development secret
  is used. Compose passes the same value to both APIs.
- Missing, malformed, expired or wrongly signed tokens: `401` with
  `WWW-Authenticate: Bearer`. The reason is logged by type only, never the token.

Rejected: a temporary `X-User-Id` header. It would be trivially spoofable, and a real
token already exists.

Consequence: the two services share a symmetric secret. Moving to asymmetric keys
(RS256/EdDSA with a public key for Analytics) is a later improvement and needs a
change in the Go API.

## Decision 2: summary contract

```http
GET /api/v1/analytics/summary?from=2026-11-01&to=2026-11-30
Authorization: Bearer <token>
```

- `from` and `to`: required, ISO dates (`YYYY-MM-DD`), **both inclusive**, interpreted
  as calendar days in `ANALYTICS_METRICS_TIMEZONE` (ADR 0001).
- `from <= to`, and the period is at most **366 days**.

```json
{
  "period": {
    "from": "2026-11-01",
    "to": "2026-11-30",
    "timezone": "America/Sao_Paulo"
  },
  "studied_seconds": 75000,
  "completed_cycles": 33,
  "completed_tasks": 18,
  "active_days": 16,
  "streak": {
    "current_days": 7,
    "longest_days": 9,
    "last_active_date": "2026-11-20"
  }
}
```

| Field | Scope | Meaning |
| --- | --- | --- |
| `studied_seconds`, `completed_cycles`, `completed_tasks` | period | sums over `user_daily_metrics` |
| `active_days` | period | days with at least one finished cycle |
| `streak.current_days` | **today** | consecutive active days ending today or yesterday; `0` otherwise. Not affected by the period |
| `streak.longest_days` | all time | longest run ever. Not affected by the period |
| `streak.last_active_date` | all time | `null` when the user never studied |

- A user with no data gets **`200` with zeros** and `last_active_date: null`, never `404`.
- "Today" is the current date in `ANALYTICS_METRICS_TIMEZONE`.

Open question for the frontend: the Go endpoints use `timeframe` + `reference_date`.
This contract uses `from` + `to` as the onboarding specifies; a client can derive
"this week/month" ranges itself. Confirm before freezing v1.

## Decision 3: error convention

Bodies follow the Go API shape so the frontend handles one format:

| Situation | Status | Body |
| --- | --- | --- |
| No/invalid/expired token | `401` | `{"error": "unauthorized"}` |
| Missing or badly formatted parameter | `422` | `{"error": "invalid request", "details": [{"field": "from", "message": "..."}]}` |
| `from` after `to`, or period over 366 days | `422` | same shape as above |
| Unexpected failure | `500` | `{"error": "internal server error"}` |
| Database unavailable during a query | `503` | `{"error": "service unavailable"}` |

- `422` (not `400`) for every validation problem, including inverted ranges, so
  "input is wrong" always has one status.
- Responses never include stack traces, SQL, connection strings or exception
  messages. Full details go to the logs (without secrets) only.

## Consequences

- `JWT_SECRET` must be set in Compose for both services; the Go API's silent default
  secret becomes visible as a risk and should be removed there too.
- All later endpoints (`daily`, `subjects`, `sprints/{id}`) reuse the same auth
  dependency, period validation and error handlers.
- Changing a field's meaning after the frontend adopts it requires `/api/v2`.
