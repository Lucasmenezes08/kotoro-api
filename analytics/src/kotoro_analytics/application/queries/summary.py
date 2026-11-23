"""Dashboard summary query (ADR 0002, decision 2).

This module defines what the summary needs from storage. It knows nothing about
HTTP or SQL: the router calls it, and an infrastructure repository implements the port.
"""

import uuid
from collections.abc import Callable
from dataclasses import dataclass
from datetime import UTC, date, datetime
from typing import Protocol
from zoneinfo import ZoneInfo

from kotoro_analytics.domain.metrics import Streak, current_streak_on
from kotoro_analytics.domain.period import Period


@dataclass(frozen=True)
class PeriodTotals:
    studied_seconds: int
    completed_cycles: int
    completed_tasks: int
    active_days: int

    @classmethod
    def empty(cls) -> "PeriodTotals":
        return cls(studied_seconds=0, completed_cycles=0, completed_tasks=0, active_days=0)


class SummaryReadRepository(Protocol):
    """Read port for the summary. Every method is scoped to one user."""

    async def period_totals(self, user_id: uuid.UUID, period: Period) -> PeriodTotals:
        """Sums over the user's active days inside `period` (both ends included)."""
        ...

    async def streak(self, user_id: uuid.UUID) -> Streak:
        """Stored streak; a user who never studied gets an empty streak."""
        ...


@dataclass(frozen=True)
class Summary:
    period: Period
    timezone: str
    totals: PeriodTotals
    current_streak_days: int
    longest_streak_days: int
    last_active_date: date | None


Clock = Callable[[], datetime]


def utc_now() -> datetime:
    return datetime.now(UTC)


class GetSummary:
    """Builds the dashboard summary of one user for one period."""

    def __init__(
        self,
        repository: SummaryReadRepository,
        *,
        timezone: ZoneInfo,
        clock: Clock = utc_now,
    ) -> None:
        self._repository = repository
        self._timezone = timezone
        self._clock = clock

    async def execute(self, user_id: uuid.UUID, period: Period) -> Summary:
        # Sequential on purpose: both calls share one AsyncSession (ADR 0001).
        totals = await self._repository.period_totals(user_id, period)
        streak = await self._repository.streak(user_id)

        # "Today" is the calendar day in the metrics time zone, like metric_date.
        today = self._clock().astimezone(self._timezone).date()
        return Summary(
            period=period,
            timezone=self._timezone.key,
            totals=totals,
            current_streak_days=current_streak_on(streak, today),
            longest_streak_days=streak.longest_days,
            last_active_date=streak.last_active_date,
        )
