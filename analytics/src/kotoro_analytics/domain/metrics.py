"""Pure metric rules shared by seeds, the consumer and the queries.

See docs/adr/0001-read-database-decisions.md (decision 3).
"""

from collections.abc import Iterable
from dataclasses import dataclass
from datetime import date, datetime, timedelta
from zoneinfo import ZoneInfo


def metric_date_for(ended_at: datetime, timezone: ZoneInfo) -> date:
    """Calendar day a finished cycle counts for: `ended_at` converted to `timezone`."""
    if ended_at.tzinfo is None:
        raise ValueError("ended_at must be timezone-aware")
    return ended_at.astimezone(timezone).date()


@dataclass(frozen=True)
class Streak:
    current_days: int
    longest_days: int
    last_active_date: date | None


def compute_streak(active_dates: Iterable[date]) -> Streak:
    """Streaks of consecutive active days.

    `current_days` is the run that ends on `last_active_date`. Whether that run is
    still alive depends on "today" and is decided when reading (see `current_streak_on`).
    """
    days = sorted(set(active_dates))
    if not days:
        return Streak(current_days=0, longest_days=0, last_active_date=None)

    longest = run = 1
    for previous, current in zip(days, days[1:], strict=False):
        run = run + 1 if current - previous == timedelta(days=1) else 1
        longest = max(longest, run)

    return Streak(current_days=run, longest_days=longest, last_active_date=days[-1])


def current_streak_on(streak: Streak, today: date) -> int:
    """Current streak as seen on `today`: alive if the last active day is today or yesterday."""
    if streak.last_active_date is None:
        return 0
    if today - streak.last_active_date <= timedelta(days=1):
        return streak.current_days
    return 0
