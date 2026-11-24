import uuid
from datetime import date
from zoneinfo import ZoneInfo

import pytest
from tests.support.fakes import FakeDailyRepository

from kotoro_analytics.application.queries.daily import DayMetrics, GetDailySeries
from kotoro_analytics.domain.period import Period

USER = uuid.UUID("11111111-1111-4111-8111-111111111111")


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


def _query(*stored: DayMetrics) -> GetDailySeries:
    return GetDailySeries(
        FakeDailyRepository(stored=list(stored)), timezone=ZoneInfo("America/Sao_Paulo")
    )


@pytest.mark.anyio
async def test_fills_inactive_days_with_zeros() -> None:
    query = _query(
        DayMetrics(date(2026, 11, 12), 4500, 2, 1),
        DayMetrics(date(2026, 11, 14), 4500, 2, 1),
    )

    series = await query.execute(USER, Period(date(2026, 11, 12), date(2026, 11, 14)))

    assert series.days == [
        DayMetrics(date(2026, 11, 12), 4500, 2, 1),
        DayMetrics.empty(date(2026, 11, 13)),
        DayMetrics(date(2026, 11, 14), 4500, 2, 1),
    ]


@pytest.mark.anyio
async def test_one_entry_per_day_even_without_any_activity() -> None:
    series = await _query().execute(USER, Period(date(2026, 11, 1), date(2026, 11, 30)))

    assert len(series.days) == 30
    assert all(day == DayMetrics.empty(day.day) for day in series.days)


@pytest.mark.anyio
async def test_sorts_ascending_whatever_the_repository_order() -> None:
    query = _query(
        DayMetrics(date(2026, 11, 3), 30, 1, 0),
        DayMetrics(date(2026, 11, 1), 10, 1, 0),
    )

    series = await query.execute(USER, Period(date(2026, 11, 1), date(2026, 11, 3)))

    assert [day.day for day in series.days] == [
        date(2026, 11, 1),
        date(2026, 11, 2),
        date(2026, 11, 3),
    ]


@pytest.mark.anyio
async def test_ignores_rows_outside_the_period() -> None:
    query = _query(DayMetrics(date(2026, 10, 31), 999, 9, 9))

    series = await query.execute(USER, Period(date(2026, 11, 1), date(2026, 11, 1)))

    assert series.days == [DayMetrics.empty(date(2026, 11, 1))]


@pytest.mark.anyio
async def test_reports_period_and_timezone() -> None:
    period = Period(date(2026, 11, 1), date(2026, 11, 2))

    series = await _query().execute(USER, period)

    assert (series.period, series.timezone) == (period, "America/Sao_Paulo")
