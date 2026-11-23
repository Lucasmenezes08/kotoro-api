import uuid
from dataclasses import dataclass, field
from datetime import UTC, datetime

import pytest
from sqlalchemy import exc as sa_exc
from tests.support.events import event_json

from kotoro_analytics.consumer.handler import HANDLED_ROUTING_KEY, Outcome, handle_message
from kotoro_analytics.domain.events import StudyCycleFinishedEventV1

USER = uuid.UUID("11111111-1111-4111-8111-111111111111")
VALID = event_json(USER, started_at=datetime(2026, 11, 20, 15, 0, tzinfo=UTC), elapsed_seconds=60)


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


@dataclass
class FakeProjector:
    result: bool = True
    error: Exception | None = None
    applied: list[StudyCycleFinishedEventV1] = field(default_factory=list)

    async def apply(self, event: StudyCycleFinishedEventV1) -> bool:
        self.applied.append(event)
        if self.error is not None:
            raise self.error
        return self.result


async def _handle(
    projector: FakeProjector,
    *,
    body: bytes = VALID,
    routing_key: str | None = HANDLED_ROUTING_KEY,
    redelivered: bool = False,
) -> Outcome:
    return await handle_message(
        body=body,
        routing_key=routing_key,
        message_id="m-1",
        redelivered=redelivered,
        projector=projector,
    )


@pytest.mark.anyio
async def test_applied_event_is_acked() -> None:
    projector = FakeProjector()

    assert await _handle(projector) is Outcome.ACK
    assert [event.user_id for event in projector.applied] == [USER]


@pytest.mark.anyio
async def test_duplicate_event_is_acked_so_it_leaves_the_queue() -> None:
    assert await _handle(FakeProjector(result=False), redelivered=True) is Outcome.ACK


@pytest.mark.anyio
@pytest.mark.parametrize(
    "body", [b"not json", b"{}", VALID.replace(b'"event_version": 1', b'"event_version": 2')]
)
async def test_invalid_message_is_rejected_without_touching_the_database(body: bytes) -> None:
    projector = FakeProjector()

    assert await _handle(projector, body=body) is Outcome.REJECT
    assert projector.applied == []


@pytest.mark.anyio
async def test_other_routing_keys_are_skipped() -> None:
    projector = FakeProjector()

    assert await _handle(projector, routing_key="sprint.v1") is Outcome.ACK
    assert projector.applied == []


@pytest.mark.anyio
@pytest.mark.parametrize(
    "error",
    [
        ConnectionRefusedError(),
        TimeoutError(),
        sa_exc.OperationalError("SELECT 1", {}, Exception()),
    ],
    ids=["refused", "timeout", "operational"],
)
async def test_database_unavailable_is_retried_even_when_redelivered(error: Exception) -> None:
    projector = FakeProjector(error=error)

    assert await _handle(projector, redelivered=False) is Outcome.RETRY
    assert await _handle(projector, redelivered=True) is Outcome.RETRY


@pytest.mark.anyio
async def test_unexpected_error_is_retried_once_then_rejected() -> None:
    projector = FakeProjector(error=RuntimeError("bug"))

    assert await _handle(projector, redelivered=False) is Outcome.RETRY
    assert await _handle(projector, redelivered=True) is Outcome.REJECT
