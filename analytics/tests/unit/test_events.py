import json
import uuid
from datetime import UTC, datetime

import pytest
from tests.support.events import event_dict

from kotoro_analytics.domain.events import InvalidEventError, parse_study_cycle_finished

USER = uuid.UUID("11111111-1111-4111-8111-111111111111")
START = datetime(2026, 11, 20, 12, 0, tzinfo=UTC)

# Real message captured from the queue (apostila Entrega 2, cap. 20).
CAPTURED_FROM_RABBITMQ = b"""{
  "id": "65d1110c-3eca-42fb-aeea-1765d3f2781b",
  "user_id": "6b6a9a6e-6084-4121-851e-9f575eaa94e3",
  "aggregate_type": "study_cycle",
  "aggregate_id": "84819d63-1623-49a8-b97a-492924cecfae",
  "event_type": "study_cycle.finished",
  "event_version": 1,
  "payload": {
    "cycle_id": "84819d63-1623-49a8-b97a-492924cecfae",
    "ended_at": "2026-11-18T23:16:42.456701Z",
    "sprint_id": "d848bbfc-8815-44ef-a87a-15389dbab255",
    "started_at": "2026-11-18T23:16:39.427357Z",
    "subject_id": "ffbff69a-67b1-472f-a468-3cec01b9fb22",
    "timer_type": "cronometro_livre",
    "stop_reason": "concluido",
    "execution_mode": "bloco_materia",
    "elapsed_seconds": 3,
    "sequence_number": 1,
    "sprint_block_id": "03f6aa1a-e757-47fe-8193-c23366c81889",
    "completed_task_ids": []
  },
  "occurred_at": "2026-11-18T23:16:42.456701Z"
}"""


def _json(**changes: object) -> bytes:
    body = event_dict(USER, started_at=START, elapsed_seconds=1500)
    for path, value in changes.items():
        target = body
        *parents, leaf = path.split("__")
        for parent in parents:
            target = target[parent]
        target[leaf] = value
    return json.dumps(body).encode()


def test_parses_a_message_captured_from_rabbitmq() -> None:
    event = parse_study_cycle_finished(CAPTURED_FROM_RABBITMQ)

    assert event.id == uuid.UUID("65d1110c-3eca-42fb-aeea-1765d3f2781b")
    assert event.payload.elapsed_seconds == 3
    assert event.payload.completed_task_ids == []
    assert event.payload.ended_at.tzinfo is not None


def test_unknown_extra_fields_are_ignored() -> None:
    body = json.loads(_json())
    body["payload"]["new_optional_field"] = "added later in v1"
    body["trace"] = {"span": 1}

    assert parse_study_cycle_finished(json.dumps(body)).user_id == USER


@pytest.mark.parametrize(
    "changes",
    [
        {"event_type": "study_cycle.cancelled"},
        {"event_version": 2},
        {"aggregate_type": "sprint"},
        {"aggregate_id": str(uuid.uuid4())},
        {"payload__elapsed_seconds": -1},
        {"payload__ended_at": "2026-11-20T11:00:00Z"},
        {"payload__started_at": "2026-11-20T12:00:00"},
        {"payload__subject_id": "not-a-uuid"},
        {"user_id": None},
    ],
    ids=[
        "other-event",
        "other-version",
        "other-aggregate",
        "aggregate-is-not-the-cycle",
        "negative-elapsed",
        "ends-before-start",
        "naive-datetime",
        "bad-uuid",
        "missing-user",
    ],
)
def test_contract_violations_are_rejected(changes: dict[str, object]) -> None:
    with pytest.raises(InvalidEventError):
        parse_study_cycle_finished(_json(**changes))


@pytest.mark.parametrize("body", [b"", b"not json", b"[]", b"{}"])
def test_malformed_bodies_are_rejected(body: bytes) -> None:
    with pytest.raises(InvalidEventError):
        parse_study_cycle_finished(body)
