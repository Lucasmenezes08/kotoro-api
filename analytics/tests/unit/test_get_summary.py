import uuid
from datetime import UTC, date, datetime
from zoneinfo import ZoneInfo

import pytest
from tests.support.fakes import FakeSummaryRepository

from kotoro_analytics.application.queries.summary import GetSummary, PeriodTotals
from kotoro_analytics.domain.metrics import Streak
from kotoro_analytics.domain.period import Period

USER = uuid.UUID("11111111-1111-4111-8111-111111111111")
NOVEMBER = Period(date(2026, 11, 1), date(2026, 11, 30))
SAO_PAULO = ZoneInfo("America/Sao_Paulo")
STORED = Streak(current_days=7, longest_days=9, last_active_date=date(2026, 11, 20))


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


def _query(repository: FakeSummaryRepository, now: datetime) -> GetSummary:
    return GetSummary(repository, timezone=SAO_PAULO, clock=lambda: now)


@pytest.mark.anyio
async def test_combines_period_totals_and_streak() -> None:
    repository = FakeSummaryRepository(
        totals=PeriodTotals(75_000, 33, 18, 16), stored_streak=STORED
    )

    summary = await _query(repository, datetime(2026, 11, 20, 15, 0, tzinfo=UTC)).execute(
        USER, NOVEMBER
    )

    assert summary.totals == PeriodTotals(75_000, 33, 18, 16)
    assert summary.period == NOVEMBER
    assert summary.timezone == "America/Sao_Paulo"
    assert (summary.current_streak_days, summary.longest_streak_days) == (7, 9)
    assert summary.last_active_date == date(2026, 11, 20)


@pytest.mark.anyio
async def test_queries_only_the_given_user() -> None:
    repository = FakeSummaryRepository()

    await _query(repository, datetime(2026, 11, 20, tzinfo=UTC)).execute(USER, NOVEMBER)

    assert {user_id for _, user_id, _ in repository.calls} == {USER}


@pytest.mark.parametrize(
    ("now", "expected_current"),
    [
        (datetime(2026, 11, 21, 12, 0, tzinfo=UTC), 7),  # yesterday was active: alive
        (datetime(2026, 11, 22, 2, 59, tzinfo=UTC), 7),  # 23:59 on Nov 21 in Sao Paulo
        (datetime(2026, 11, 22, 3, 0, tzinfo=UTC), 0),  # 00:00 on Nov 22 in Sao Paulo
    ],
    ids=["next-day", "last-minute-local", "first-minute-after"],
)
@pytest.mark.anyio
async def test_current_streak_uses_today_in_metrics_timezone(
    now: datetime, expected_current: int
) -> None:
    repository = FakeSummaryRepository(stored_streak=STORED)

    summary = await _query(repository, now).execute(USER, NOVEMBER)

    assert summary.current_streak_days == expected_current
    assert summary.longest_streak_days == 9


@pytest.mark.anyio
async def test_user_without_activity_gets_zeros() -> None:
    summary = await _query(FakeSummaryRepository(), datetime.now(UTC)).execute(USER, NOVEMBER)

    assert summary.totals == PeriodTotals.empty()
    assert (summary.current_streak_days, summary.longest_streak_days) == (0, 0)
    assert summary.last_active_date is None
