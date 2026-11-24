"""Daily series for charts (ADR 0003, decision 1)."""

import uuid
from dataclasses import dataclass
from datetime import date, timedelta
from typing import Protocol
from zoneinfo import ZoneInfo

from kotoro_analytics.domain.period import Period


@dataclass(frozen=True)
class DayMetrics:
    day: date
    studied_seconds: int
    completed_cycles: int
    completed_tasks: int

    @classmethod
    def empty(cls, day: date) -> "DayMetrics":
        return cls(day=day, studied_seconds=0, completed_cycles=0, completed_tasks=0)


class DailyReadRepository(Protocol):
    async def active_days(self, user_id: uuid.UUID, period: Period) -> list[DayMetrics]:
        """Days of `period` on which the user has metrics. Inactive days are absent."""
        ...


@dataclass(frozen=True)
class DailySeries:
    period: Period
    timezone: str
    days: list[DayMetrics]


class GetDailySeries:
    def __init__(self, repository: DailyReadRepository, *, timezone: ZoneInfo) -> None:
        self._repository = repository
        self._timezone = timezone

    async def execute(self, user_id: uuid.UUID, period: Period) -> DailySeries:
        stored = {
            metrics.day: metrics
            for metrics in await self._repository.active_days(user_id, period)
            if period.start <= metrics.day <= period.end
        }
        # One entry per calendar day, inactive days as zeros, ascending (ADR 0003).
        days = [
            stored.get(day, DayMetrics.empty(day))
            for day in (period.start + timedelta(days=offset) for offset in range(period.days))
        ]
        return DailySeries(period=period, timezone=self._timezone.key, days=days)
