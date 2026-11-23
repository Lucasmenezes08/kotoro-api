"""PostgresSummaryRepository against the seeded test database.

Expected numbers come from kotoro_analytics.seeds (documented in analytics/README.md).
"""

import uuid
from collections.abc import AsyncIterator
from datetime import date
from zoneinfo import ZoneInfo

import pytest
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncEngine, AsyncSession

from kotoro_analytics.application.queries.summary import PeriodTotals, SummaryReadRepository
from kotoro_analytics.config import Settings
from kotoro_analytics.domain.metrics import Streak
from kotoro_analytics.domain.period import Period
from kotoro_analytics.infrastructure.database.repositories import PostgresSummaryRepository
from kotoro_analytics.infrastructure.database.session import (
    create_engine,
    create_session_factory,
)
from kotoro_analytics.seeds import (
    DEFAULT_END_DATE,
    USER_ANA,
    USER_BRUNO,
    USER_CARLA,
    apply_seed,
    build_seed_dataset,
)

pytestmark = pytest.mark.anyio

NOVEMBER = Period(date(2026, 11, 1), date(2026, 11, 30))
UNKNOWN_USER = uuid.UUID("44444444-4444-4444-8444-444444444444")


@pytest.fixture
async def session(
    writer_engine: AsyncEngine, migrated_database: str
) -> AsyncIterator[AsyncSession]:
    """Seeded database, read through the API's own read-only engine."""
    await apply_seed(
        writer_engine, build_seed_dataset(DEFAULT_END_DATE, ZoneInfo("America/Sao_Paulo"))
    )

    engine = create_engine(Settings(environment="test", database_url=migrated_database))
    try:
        async with create_session_factory(engine)() as read_only_session:
            yield read_only_session
    finally:
        await engine.dispose()


@pytest.fixture
def repository(session: AsyncSession) -> SummaryReadRepository:
    return PostgresSummaryRepository(session)


@pytest.mark.parametrize(
    ("period", "expected"),
    [
        (NOVEMBER, PeriodTotals(75_000, 33, 18, 16)),
        # Nov 4-12: the 9-day run before the gap.
        (Period(date(2026, 11, 4), date(2026, 11, 12)), PeriodTotals(40_500, 18, 9, 9)),
        # Nov 20 includes the 22:00 cycle that ends on Nov 21 in UTC.
        (Period(date(2026, 11, 20), date(2026, 11, 20)), PeriodTotals(7_500, 3, 3, 1)),
        # The gap day.
        (Period(date(2026, 11, 13), date(2026, 11, 13)), PeriodTotals.empty()),
        # Before any activity.
        (Period(date(2026, 10, 1), date(2026, 10, 31)), PeriodTotals.empty()),
    ],
    ids=["november", "first-run", "late-night-day", "gap-day", "october"],
)
async def test_ana_period_totals(
    repository: SummaryReadRepository, period: Period, expected: PeriodTotals
) -> None:
    assert await repository.period_totals(USER_ANA, period) == expected


async def test_bruno_sees_only_his_data(repository: SummaryReadRepository) -> None:
    # Same days as Ana, very different totals: isolation comes from user_id.
    assert await repository.period_totals(USER_BRUNO, NOVEMBER) == PeriodTotals(8_100, 3, 6, 3)


@pytest.mark.parametrize("user_id", [USER_CARLA, UNKNOWN_USER], ids=["carla", "unknown"])
async def test_user_without_activity_gets_zeros(
    repository: SummaryReadRepository, user_id: uuid.UUID
) -> None:
    assert await repository.period_totals(user_id, NOVEMBER) == PeriodTotals.empty()
    assert await repository.streak(user_id) == Streak(0, 0, None)


async def test_ana_streak(repository: SummaryReadRepository) -> None:
    assert await repository.streak(USER_ANA) == Streak(7, 9, DEFAULT_END_DATE)


async def test_bruno_streak(repository: SummaryReadRepository) -> None:
    assert await repository.streak(USER_BRUNO) == Streak(3, 3, DEFAULT_END_DATE)


async def test_period_query_can_use_primary_key_index(session: AsyncSession) -> None:
    # Tables are tiny in tests, so the planner would pick a seq scan anyway.
    # Disabling it proves the filter matches pk_user_daily_metrics (user_id, metric_date).
    await session.execute(text("SET LOCAL enable_seqscan = off"))
    plan = (
        (
            await session.execute(
                text(
                    "EXPLAIN SELECT sum(studied_seconds) FROM user_daily_metrics "
                    "WHERE user_id = :user_id AND metric_date BETWEEN :start AND :end"
                ),
                {"user_id": USER_ANA, "start": NOVEMBER.start, "end": NOVEMBER.end},
            )
        )
        .scalars()
        .all()
    )

    assert any("pk_user_daily_metrics" in line for line in plan), "\n".join(plan)
