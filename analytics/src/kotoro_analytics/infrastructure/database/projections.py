"""Applies `study_cycle.finished.v1` to the read model (the consumer's write side).

Guarantees, all inside ONE transaction per event:
- idempotent: `processed_events` is written first with ON CONFLICT DO NOTHING; a
  redelivered event changes nothing;
- order-independent: totals are additive upserts, sprint first/last use LEAST/GREATEST,
  and the streak is recomputed from the stored active days;
- safe with parallel consumers: a per-user advisory lock serialises events of the same
  user, so two transactions never recompute that user's streak from stale days.
"""

import logging
import uuid
from zoneinfo import ZoneInfo

from sqlalchemy import func, select
from sqlalchemy.dialects.postgresql import insert
from sqlalchemy.ext.asyncio import AsyncConnection, AsyncEngine

from kotoro_analytics.domain.events import StudyCycleFinishedEventV1
from kotoro_analytics.domain.metrics import compute_streak, metric_date_for
from kotoro_analytics.infrastructure.database.models import (
    ProcessedEvent,
    SprintSummary,
    StudyCycleFact,
    UserDailyMetric,
    UserStreak,
    UserSubjectDailyMetric,
)

logger = logging.getLogger(__name__)


def _advisory_lock_key(user_id: uuid.UUID) -> int:
    """Signed 64-bit key derived from the user id, as pg_advisory_xact_lock expects."""
    key = user_id.int >> 64
    return key - (1 << 64) if key >= (1 << 63) else key


def _additive_upsert(model: type, index_elements: list[str], values: dict) -> object:
    statement = insert(model).values(**values)
    excluded = statement.excluded
    return statement.on_conflict_do_update(
        index_elements=index_elements,
        set_={
            "studied_seconds": model.studied_seconds + excluded.studied_seconds,
            "completed_cycles": model.completed_cycles + excluded.completed_cycles,
            "completed_tasks": model.completed_tasks + excluded.completed_tasks,
            "updated_at": func.now(),
        },
    )


class StudyCycleFinishedProjector:
    def __init__(self, engine: AsyncEngine, *, timezone: ZoneInfo) -> None:
        self._engine = engine
        self._timezone = timezone

    async def apply(self, event: StudyCycleFinishedEventV1) -> bool:
        """Project the event. Returns False when it had already been applied."""
        async with self._engine.begin() as connection:
            await connection.execute(
                select(func.pg_advisory_xact_lock(_advisory_lock_key(event.user_id)))
            )
            if not await self._mark_processed(connection, event):
                logger.info("duplicate event ignored: %s", event.id)
                return False
            if not await self._insert_fact(connection, event):
                # Same cycle under another event id: count it once (fact is keyed by cycle).
                logger.warning(
                    "cycle %s already projected; event %s skipped", event.payload.cycle_id, event.id
                )
                return False
            await self._add_to_totals(connection, event)
            await self._recompute_streak(connection, event.user_id)
        return True

    async def _mark_processed(
        self, connection: AsyncConnection, event: StudyCycleFinishedEventV1
    ) -> bool:
        result = await connection.execute(
            insert(ProcessedEvent)
            .values(
                event_id=event.id,
                event_type=event.event_type,
                event_version=event.event_version,
                occurred_at=event.occurred_at,
            )
            .on_conflict_do_nothing(index_elements=["event_id"])
            .returning(ProcessedEvent.event_id)
        )
        return result.first() is not None

    async def _insert_fact(
        self, connection: AsyncConnection, event: StudyCycleFinishedEventV1
    ) -> bool:
        payload = event.payload
        result = await connection.execute(
            insert(StudyCycleFact)
            .values(
                cycle_id=payload.cycle_id,
                event_id=event.id,
                user_id=event.user_id,
                sprint_id=payload.sprint_id,
                sprint_block_id=payload.sprint_block_id,
                subject_id=payload.subject_id,
                sequence_number=payload.sequence_number,
                execution_mode=payload.execution_mode,
                timer_type=payload.timer_type,
                stop_reason=payload.stop_reason,
                started_at=payload.started_at,
                ended_at=payload.ended_at,
                elapsed_seconds=payload.elapsed_seconds,
                completed_task_count=len(payload.completed_task_ids),
                metric_date=metric_date_for(payload.ended_at, self._timezone),
            )
            .on_conflict_do_nothing(index_elements=["cycle_id"])
            .returning(StudyCycleFact.cycle_id)
        )
        return result.first() is not None

    async def _add_to_totals(
        self, connection: AsyncConnection, event: StudyCycleFinishedEventV1
    ) -> None:
        payload = event.payload
        day = metric_date_for(payload.ended_at, self._timezone)
        increment = {
            "studied_seconds": payload.elapsed_seconds,
            "completed_cycles": 1,
            "completed_tasks": len(payload.completed_task_ids),
        }

        await connection.execute(
            _additive_upsert(
                UserDailyMetric,
                ["user_id", "metric_date"],
                {"user_id": event.user_id, "metric_date": day, **increment},
            )
        )
        await connection.execute(
            _additive_upsert(
                UserSubjectDailyMetric,
                ["user_id", "metric_date", "subject_id"],
                {
                    "user_id": event.user_id,
                    "metric_date": day,
                    "subject_id": payload.subject_id,
                    **increment,
                },
            )
        )

        sprint = insert(SprintSummary).values(
            sprint_id=payload.sprint_id,
            user_id=event.user_id,
            first_cycle_started_at=payload.started_at,
            last_cycle_ended_at=payload.ended_at,
            **increment,
        )
        await connection.execute(
            sprint.on_conflict_do_update(
                index_elements=["sprint_id"],
                set_={
                    "studied_seconds": SprintSummary.studied_seconds
                    + sprint.excluded.studied_seconds,
                    "completed_cycles": SprintSummary.completed_cycles
                    + sprint.excluded.completed_cycles,
                    "completed_tasks": SprintSummary.completed_tasks
                    + sprint.excluded.completed_tasks,
                    "first_cycle_started_at": func.least(
                        SprintSummary.first_cycle_started_at, sprint.excluded.first_cycle_started_at
                    ),
                    "last_cycle_ended_at": func.greatest(
                        SprintSummary.last_cycle_ended_at, sprint.excluded.last_cycle_ended_at
                    ),
                    "updated_at": func.now(),
                },
                # A sprint belongs to one user; never merge another user's cycles into it.
                where=SprintSummary.user_id == sprint.excluded.user_id,
            )
        )

    async def _recompute_streak(self, connection: AsyncConnection, user_id: uuid.UUID) -> None:
        active_days = (
            await connection.execute(
                select(UserDailyMetric.metric_date).where(
                    UserDailyMetric.user_id == user_id, UserDailyMetric.completed_cycles > 0
                )
            )
        ).scalars()
        streak = compute_streak(active_days)

        statement = insert(UserStreak).values(
            user_id=user_id,
            current_streak_days=streak.current_days,
            longest_streak_days=streak.longest_days,
            last_active_date=streak.last_active_date,
        )
        await connection.execute(
            statement.on_conflict_do_update(
                index_elements=["user_id"],
                set_={
                    "current_streak_days": statement.excluded.current_streak_days,
                    "longest_streak_days": statement.excluded.longest_streak_days,
                    "last_active_date": statement.excluded.last_active_date,
                    "updated_at": func.now(),
                },
            )
        )
