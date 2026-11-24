import uuid
from datetime import date
from zoneinfo import ZoneInfo

import pytest
from sqlalchemy import func, insert, select, text
from sqlalchemy.exc import DBAPIError
from sqlalchemy.ext.asyncio import AsyncEngine

from kotoro_analytics.config import Settings
from kotoro_analytics.infrastructure.database.models import (
    ProcessedEvent,
    StudyCycleFact,
    UserDailyMetric,
    UserStreak,
)
from kotoro_analytics.infrastructure.database.session import (
    create_engine,
    create_session_factory,
)
from kotoro_analytics.seeds import (
    DEFAULT_END_DATE,
    SEED_USERS,
    USER_ANA,
    USER_BRUNO,
    apply_seed,
    build_seed_dataset,
)

pytestmark = pytest.mark.anyio

SAO_PAULO = ZoneInfo("America/Sao_Paulo")
OTHER_USER = uuid.UUID("99999999-9999-4999-8999-999999999999")


async def _daily_totals(engine: AsyncEngine, user_id: uuid.UUID) -> tuple[int, int]:
    async with engine.connect() as connection:
        row = (
            await connection.execute(
                select(
                    func.coalesce(func.sum(UserDailyMetric.studied_seconds), 0),
                    func.coalesce(func.sum(UserDailyMetric.completed_cycles), 0),
                ).where(UserDailyMetric.user_id == user_id)
            )
        ).one()
    return int(row[0]), int(row[1])


async def test_seed_writes_expected_totals(writer_engine: AsyncEngine) -> None:
    await apply_seed(writer_engine, build_seed_dataset(DEFAULT_END_DATE, SAO_PAULO))

    assert await _daily_totals(writer_engine, USER_ANA) == (75_000, 33)
    assert await _daily_totals(writer_engine, USER_BRUNO) == (8_100, 3)

    async with writer_engine.connect() as connection:
        streak = (
            await connection.execute(select(UserStreak).where(UserStreak.user_id == USER_ANA))
        ).one()
    assert (streak.current_streak_days, streak.longest_streak_days) == (7, 9)


async def test_seed_is_idempotent_and_keeps_other_users(writer_engine: AsyncEngine) -> None:
    async with writer_engine.begin() as connection:
        await connection.execute(
            text("DELETE FROM user_daily_metrics WHERE user_id = :user_id"),
            {"user_id": OTHER_USER},
        )
        await connection.execute(
            insert(UserDailyMetric),
            [{"user_id": OTHER_USER, "metric_date": date(2026, 11, 1), "studied_seconds": 60}],
        )

    dataset = build_seed_dataset(DEFAULT_END_DATE, SAO_PAULO)
    await apply_seed(writer_engine, dataset)
    await apply_seed(writer_engine, dataset)

    async with writer_engine.connect() as connection:
        # Scoped to the seed: other tests share this database.
        facts = await connection.scalar(
            select(func.count())
            .select_from(StudyCycleFact)
            .where(StudyCycleFact.user_id.in_(SEED_USERS))
        )
        events = await connection.scalar(
            select(func.count())
            .select_from(ProcessedEvent)
            .where(ProcessedEvent.event_id.in_([e["event_id"] for e in dataset.processed_events]))
        )

    assert facts == len(dataset.facts)
    assert events == len(dataset.processed_events)
    assert await _daily_totals(writer_engine, OTHER_USER) == (60, 0)


async def test_late_night_cycle_is_stored_on_local_day(writer_engine: AsyncEngine) -> None:
    await apply_seed(writer_engine, build_seed_dataset(DEFAULT_END_DATE, SAO_PAULO))

    async with writer_engine.connect() as connection:
        row = (
            await connection.execute(
                select(StudyCycleFact.metric_date, StudyCycleFact.ended_at).where(
                    StudyCycleFact.user_id == USER_ANA, StudyCycleFact.sequence_number == 3
                )
            )
        ).one()

    assert row.metric_date == DEFAULT_END_DATE
    assert row.ended_at.astimezone(ZoneInfo("UTC")).date() == date(2026, 11, 21)


async def test_api_session_cannot_write(migrated_database: str) -> None:
    engine = create_engine(Settings(database_url=migrated_database))
    try:
        async with create_session_factory(engine)() as session:
            with pytest.raises(DBAPIError, match="read-only transaction"):
                await session.execute(text("DELETE FROM user_daily_metrics"))
    finally:
        await engine.dispose()
