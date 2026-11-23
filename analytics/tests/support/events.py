"""Builds `study_cycle.finished.v1` messages shaped like the Go relay publishes them
(internal/messaging/rabbitmq/envelope.go + internal/study-cycles/events.go)."""

import json
import uuid
from datetime import UTC, datetime, timedelta
from typing import Any

from kotoro_analytics.domain.events import StudyCycleFinishedEventV1


def event_dict(
    user_id: uuid.UUID,
    *,
    started_at: datetime,
    elapsed_seconds: int,
    subject_id: uuid.UUID | None = None,
    sprint_id: uuid.UUID | None = None,
    cycle_id: uuid.UUID | None = None,
    event_id: uuid.UUID | None = None,
    completed_tasks: int = 0,
    stop_reason: str = "concluido",
    sequence_number: int = 1,
) -> dict[str, Any]:
    cycle_id = cycle_id or uuid.uuid4()
    ended_at = started_at + timedelta(seconds=elapsed_seconds)
    return {
        "id": str(event_id or uuid.uuid4()),
        "user_id": str(user_id),
        "aggregate_type": "study_cycle",
        "aggregate_id": str(cycle_id),
        "event_type": "study_cycle.finished",
        "event_version": 1,
        "payload": {
            "cycle_id": str(cycle_id),
            "sprint_id": str(sprint_id or uuid.uuid4()),
            "sprint_block_id": str(uuid.uuid4()),
            "subject_id": str(subject_id or uuid.uuid4()),
            "sequence_number": sequence_number,
            "execution_mode": "bloco_materia",
            "timer_type": "temporizado",
            "started_at": started_at.astimezone(UTC).isoformat().replace("+00:00", "Z"),
            "ended_at": ended_at.astimezone(UTC).isoformat().replace("+00:00", "Z"),
            "elapsed_seconds": elapsed_seconds,
            "stop_reason": stop_reason,
            "completed_task_ids": [str(uuid.uuid4()) for _ in range(completed_tasks)],
        },
        "occurred_at": ended_at.astimezone(UTC).isoformat().replace("+00:00", "Z"),
    }


def event_json(user_id: uuid.UUID, **kwargs: Any) -> bytes:
    return json.dumps(event_dict(user_id, **kwargs)).encode()


def make_event(user_id: uuid.UUID, **kwargs: Any) -> StudyCycleFinishedEventV1:
    return StudyCycleFinishedEventV1.model_validate(event_dict(user_id, **kwargs))
