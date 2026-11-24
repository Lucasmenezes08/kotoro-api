"""StudyCycleFinishedProjector against PostgreSQL: idempotency, order, concurrency."""

import asyncio
import random
import uuid
from datetime import UTC, date, datetime, timedelta
from zoneinfo import ZoneInfo

import pytest
from sqlalchemy import delete, select
from sqlalchemy.ext.asyncio import AsyncEngine

from kotoro_analytics.domain.events import StudyCycleFinishedEventV1
from kotoro_analytics.infrastructure.database.models import (
    ProcessedEvent,
    SprintSummary,
    StudyCycleFact,
    UserDailyMetric,
    UserStreak,
    UserSubjectDailyMetric,
)
from kotoro_analytics.infrastructure.database.projections import StudyCycleFinishedProjector
from kotoro_analytics.seeds import DEFAULT_END_DATE, SEED_USERS, build_seed_dataset
from tests.support.events import make_event

pytestmark = pytest.mark.anyio

SAO_PAULO = ZoneInfo("America/Sao_Paulo")
# 12:00 in Sao Paulo.
NOON = datetime(2026, 11, 10, 15, 0, tzinfo=UTC)


@pytest.fixture
def projector(writer_engine: AsyncEngine) -> StudyCycleFinishedProjector:
    return StudyCycleFinishedProjector(writer_engine, timezone=SAO_PAULO)


async def _rows(engine: AsyncEngine, model: type, user_id: uuid.UUID) -> list:
    async with engine.connect() as connection:
        return (await connection.execute(select(model).where(model.user_id == user_id))).all()


async def _daily(engine: AsyncEngine, user_id: uuid.UUID) -> dict[date, tuple[int, int, int]]:
    return {
        row.metric_date: (row.studied_seconds, row.completed_cycles, row.completed_tasks)
        for row in await _rows(engine, UserDailyMetric, user_id)
    }


async def _streak(engine: AsyncEngine, user_id: uuid.UUID) -> tuple[int, int, date | None]:
    [row] = await _rows(engine, UserStreak, user_id)
    return row.current_streak_days, row.longest_streak_days, row.last_active_date


async def test_one_event_updates_every_projection(
    projector: StudyCycleFinishedProjector, writer_engine: AsyncEngine
) -> None:
    user, subject, sprint = uuid.uuid4(), uuid.uuid4(), uuid.uuid4()
    event = make_event(
        user,
        started_at=NOON,
        elapsed_seconds=1_500,
        subject_id=subject,
        sprint_id=sprint,
        completed_tasks=2,
    )

    assert await projector.apply(event) is True

    day = date(2026, 11, 10)
    assert await _daily(writer_engine, user) == {day: (1_500, 1, 2)}
    [subject_row] = await _rows(writer_engine, UserSubjectDailyMetric, user)
    assert (subject_row.subject_id, subject_row.studied_seconds) == (subject, 1_500)
    [sprint_row] = await _rows(writer_engine, SprintSummary, user)
    assert (sprint_row.sprint_id, sprint_row.completed_tasks) == (sprint, 2)
    [fact] = await _rows(writer_engine, StudyCycleFact, user)
    assert (fact.event_id, fact.metric_date, fact.completed_task_count) == (event.id, day, 2)
    assert await _streak(writer_engine, user) == (1, 1, day)


async def test_redelivered_event_is_applied_once(
    projector: StudyCycleFinishedProjector, writer_engine: AsyncEngine
) -> None:
    user = uuid.uuid4()
    event = make_event(user, started_at=NOON, elapsed_seconds=1_500)

    results = [await projector.apply(event) for _ in range(3)]

    assert results == [True, False, False]
    assert await _daily(writer_engine, user) == {date(2026, 11, 10): (1_500, 1, 0)}


async def test_same_cycle_under_another_event_id_is_counted_once(
    projector: StudyCycleFinishedProjector, writer_engine: AsyncEngine
) -> None:
    user, cycle = uuid.uuid4(), uuid.uuid4()

    first = await projector.apply(
        make_event(user, started_at=NOON, elapsed_seconds=60, cycle_id=cycle)
    )
    second = await projector.apply(
        make_event(user, started_at=NOON, elapsed_seconds=60, cycle_id=cycle)
    )

    assert (first, second) == (True, False)
    assert await _daily(writer_engine, user) == {date(2026, 11, 10): (60, 1, 0)}


async def test_late_night_cycle_counts_for_the_local_day(
    projector: StudyCycleFinishedProjector, writer_engine: AsyncEngine
) -> None:
    user = uuid.uuid4()
    # 22:00-22:50 in Sao Paulo on Nov 20 = 01:00-01:50 UTC on Nov 21.
    await projector.apply(
        make_event(user, started_at=datetime(2026, 11, 21, 1, 0, tzinfo=UTC), elapsed_seconds=3_000)
    )

    assert list(await _daily(writer_engine, user)) == [date(2026, 11, 20)]


async def test_out_of_order_events_give_the_same_result(
    projector: StudyCycleFinishedProjector, writer_engine: AsyncEngine
) -> None:
    def events_for(user: uuid.UUID) -> list[StudyCycleFinishedEventV1]:
        sprint = uuid.uuid5(uuid.NAMESPACE_OID, f"sprint-{user}")
        # Days 0, 1, 2 and 4 (two cycles each): longest streak 3, current 1 as of day 4.
        return [
            make_event(
                user,
                started_at=NOON + timedelta(days=offset, hours=hour),
                elapsed_seconds=600 * (offset + 1),
                sprint_id=sprint,
                completed_tasks=offset,
                cycle_id=uuid.uuid5(uuid.NAMESPACE_OID, f"{user}-{offset}-{hour}"),
            )
            for offset in (0, 1, 2, 4)
            for hour in (0, 3)
        ]

    in_order, shuffled = uuid.uuid4(), uuid.uuid4()
    for event in events_for(in_order):
        await projector.apply(event)
    reversed_events = list(reversed(events_for(shuffled)))
    for event in reversed_events:
        await projector.apply(event)

    assert await _daily(writer_engine, in_order) == await _daily(writer_engine, shuffled)
    assert await _streak(writer_engine, in_order) == (1, 3, date(2026, 11, 14))
    assert await _streak(writer_engine, shuffled) == (1, 3, date(2026, 11, 14))

    [sprint_in_order] = await _rows(writer_engine, SprintSummary, in_order)
    [sprint_shuffled] = await _rows(writer_engine, SprintSummary, shuffled)
    for row in (sprint_in_order, sprint_shuffled):
        assert row.first_cycle_started_at == NOON
        assert row.last_cycle_ended_at == NOON + timedelta(days=4, hours=3, seconds=3_000)
        assert (row.completed_cycles, row.studied_seconds) == (8, 2 * 600 * (1 + 2 + 3 + 5))


async def test_parallel_consumers_on_the_same_user_stay_consistent(
    projector: StudyCycleFinishedProjector, writer_engine: AsyncEngine
) -> None:
    user = uuid.uuid4()
    events = [
        make_event(
            user, started_at=NOON + timedelta(days=day), elapsed_seconds=100, completed_tasks=1
        )
        for day in range(10)
    ]
    # Every event delivered twice, all at the same time.
    results = await asyncio.gather(*(projector.apply(event) for event in events + events))

    assert sum(results) == 10
    daily = await _daily(writer_engine, user)
    assert len(daily) == 10
    assert set(daily.values()) == {(100, 1, 1)}
    assert await _streak(writer_engine, user) == (10, 10, date(2026, 11, 19))


async def test_projecting_the_seed_events_rebuilds_the_seed_tables(
    projector: StudyCycleFinishedProjector, writer_engine: AsyncEngine
) -> None:
    dataset = build_seed_dataset(DEFAULT_END_DATE, SAO_PAULO)
    events = [
        StudyCycleFinishedEventV1.model_validate(
            {
                "id": fact["event_id"],
                "user_id": fact["user_id"],
                "aggregate_type": "study_cycle",
                "aggregate_id": fact["cycle_id"],
                "event_type": "study_cycle.finished",
                "event_version": 1,
                "occurred_at": fact["ended_at"],
                "payload": {
                    **{
                        key: fact[key]
                        for key in (
                            "cycle_id",
                            "sprint_id",
                            "sprint_block_id",
                            "subject_id",
                            "sequence_number",
                            "execution_mode",
                            "timer_type",
                            "started_at",
                            "ended_at",
                            "elapsed_seconds",
                            "stop_reason",
                        )
                    },
                    "completed_task_ids": [
                        uuid.uuid4() for _ in range(fact["completed_task_count"])
                    ],
                },
            }
        )
        for fact in dataset.facts
    ]

    async with writer_engine.begin() as connection:
        for model in (
            StudyCycleFact,
            UserDailyMetric,
            UserSubjectDailyMetric,
            SprintSummary,
            UserStreak,
        ):
            await connection.execute(delete(model).where(model.user_id.in_(SEED_USERS)))
        await connection.execute(
            delete(ProcessedEvent).where(ProcessedEvent.event_id.in_([e.id for e in events]))
        )

    random.Random(42).shuffle(events)
    for event in events:
        assert await projector.apply(event) is True

    def keyed(rows: list[dict], *keys: str) -> dict:
        columns = ("studied_seconds", "completed_cycles", "completed_tasks")
        return {tuple(row[k] for k in keys): tuple(row[c] for c in columns) for row in rows}

    for user in SEED_USERS[:2]:
        expected_daily = keyed([r for r in dataset.daily if r["user_id"] == user], "metric_date")
        assert {(d,): v for d, v in (await _daily(writer_engine, user)).items()} == expected_daily

        expected_subjects = keyed(
            [r for r in dataset.subject_daily if r["user_id"] == user], "metric_date", "subject_id"
        )
        actual_subjects = {
            (r.metric_date, r.subject_id): (
                r.studied_seconds,
                r.completed_cycles,
                r.completed_tasks,
            )
            for r in await _rows(writer_engine, UserSubjectDailyMetric, user)
        }
        assert actual_subjects == expected_subjects

        expected_sprints = {
            r["sprint_id"]: (
                r["studied_seconds"],
                r["first_cycle_started_at"],
                r["last_cycle_ended_at"],
            )
            for r in dataset.sprints
            if r["user_id"] == user
        }
        actual_sprints = {
            r.sprint_id: (r.studied_seconds, r.first_cycle_started_at, r.last_cycle_ended_at)
            for r in await _rows(writer_engine, SprintSummary, user)
        }
        assert actual_sprints == expected_sprints

        [expected_streak] = [s for s in dataset.streaks if s["user_id"] == user]
        assert await _streak(writer_engine, user) == (
            expected_streak["current_streak_days"],
            expected_streak["longest_streak_days"],
            expected_streak["last_active_date"],
        )
