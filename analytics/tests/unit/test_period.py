from datetime import date

import pytest

from kotoro_analytics.domain.period import MAX_PERIOD_DAYS, InvalidPeriodError, Period


def test_single_day_period_has_one_day() -> None:
    assert Period(date(2026, 11, 20), date(2026, 11, 20)).days == 1


def test_month_period_counts_both_ends() -> None:
    assert Period(date(2026, 11, 1), date(2026, 11, 30)).days == 30


def test_inverted_period_blames_from() -> None:
    with pytest.raises(InvalidPeriodError) as caught:
        Period(date(2026, 11, 30), date(2026, 11, 1))

    assert caught.value.field == "from"


def test_period_of_exactly_max_days_is_allowed() -> None:
    assert Period(date(2024, 1, 1), date(2024, 12, 31)).days == MAX_PERIOD_DAYS  # leap year


def test_period_longer_than_max_days_blames_to() -> None:
    with pytest.raises(InvalidPeriodError) as caught:
        Period(date(2024, 1, 1), date(2025, 1, 1))

    assert caught.value.field == "to"
