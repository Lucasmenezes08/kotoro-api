from datetime import UTC, date, datetime
from zoneinfo import ZoneInfo

import pytest

from kotoro_analytics.domain.metrics import (
    Streak,
    compute_streak,
    current_streak_on,
    metric_date_for,
)

SAO_PAULO = ZoneInfo("America/Sao_Paulo")


def test_late_night_cycle_counts_for_local_day() -> None:
    # 01:30 UTC on Nov 21 is 22:30 on Nov 20 in Sao Paulo.
    ended_at = datetime(2026, 11, 21, 1, 30, tzinfo=UTC)

    assert metric_date_for(ended_at, SAO_PAULO) == date(2026, 11, 20)


def test_naive_datetime_is_rejected() -> None:
    with pytest.raises(ValueError, match="timezone-aware"):
        metric_date_for(datetime(2026, 11, 20, 12, 0), SAO_PAULO)


def test_streak_of_no_days_is_zero() -> None:
    assert compute_streak([]) == Streak(0, 0, None)


def test_streak_counts_longest_and_current_runs() -> None:
    days = [date(2026, 11, d) for d in (1, 2, 3, 5, 6)]

    assert compute_streak(days) == Streak(
        current_days=2, longest_days=3, last_active_date=date(2026, 11, 6)
    )


def test_streak_ignores_duplicate_days() -> None:
    days = [date(2026, 11, 1), date(2026, 11, 1), date(2026, 11, 2)]

    assert compute_streak(days).longest_days == 2


@pytest.mark.parametrize(
    ("today", "expected"),
    [
        (date(2026, 11, 20), 4),  # active today
        (date(2026, 11, 21), 4),  # active yesterday: still alive
        (date(2026, 11, 22), 0),  # missed a day: broken
    ],
)
def test_current_streak_depends_on_today(today: date, expected: int) -> None:
    streak = Streak(current_days=4, longest_days=4, last_active_date=date(2026, 11, 20))

    assert current_streak_on(streak, today) == expected
