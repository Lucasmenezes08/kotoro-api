"""Decides what happens to one RabbitMQ message. Pure logic: no broker, testable alone.

- ACK: applied, duplicate (redelivery) or an event type this consumer does not handle.
- REJECT: can never succeed (malformed / breaks the contract). Dropped and logged; there
  is no dead-letter queue yet (same limitation as the relay, see README).
- RETRY: the database is unavailable. The message goes back to the queue.
"""

import enum
import logging
from typing import Protocol

from kotoro_analytics.domain.events import (
    InvalidEventError,
    StudyCycleFinishedEventV1,
    parse_study_cycle_finished,
)
from kotoro_analytics.infrastructure.database.errors import is_database_unavailable

logger = logging.getLogger(__name__)

HANDLED_ROUTING_KEY = "study_cycle.finished.v1"


class Outcome(enum.Enum):
    ACK = "ack"
    REJECT = "reject"
    RETRY = "retry"


class Projector(Protocol):
    async def apply(self, event: StudyCycleFinishedEventV1) -> bool: ...


async def handle_message(
    *,
    body: bytes,
    routing_key: str | None,
    message_id: str | None,
    redelivered: bool,
    projector: Projector,
) -> Outcome:
    if routing_key != HANDLED_ROUTING_KEY:
        # The queue also has a wildcard binding owned by the relay; not ours to handle.
        logger.info("skipping message %s with routing key %s", message_id, routing_key)
        return Outcome.ACK

    try:
        event = parse_study_cycle_finished(body)
    except InvalidEventError as exc:
        logger.error("rejecting invalid message %s: %s", message_id, exc)
        return Outcome.REJECT

    try:
        applied = await projector.apply(event)
    except Exception as exc:
        if is_database_unavailable(exc):
            logger.warning(
                "database unavailable, will retry event %s: %s", event.id, type(exc).__name__
            )
            return Outcome.RETRY
        if not redelivered:
            # Could be transient (e.g. a deadlock); one more attempt before giving up.
            logger.exception("failed to project event %s, retrying once", event.id)
            return Outcome.RETRY
        logger.exception("failed to project event %s again, rejecting it", event.id)
        return Outcome.REJECT

    logger.info("event %s %s", event.id, "projected" if applied else "already projected")
    return Outcome.ACK
