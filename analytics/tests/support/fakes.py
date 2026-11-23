import uuid
from dataclasses import dataclass, field

from kotoro_analytics.application.queries.daily import DayMetrics
from kotoro_analytics.application.queries.subjects import SubjectMetrics
from kotoro_analytics.application.queries.summary import PeriodTotals
from kotoro_analytics.domain.metrics import Streak
from kotoro_analytics.domain.period import Period


@dataclass
class FakeSummaryRepository:
    """In-memory SummaryReadRepository that records who was asked for what."""

    totals: PeriodTotals = field(default_factory=PeriodTotals.empty)
    stored_streak: Streak = field(default_factory=lambda: Streak(0, 0, None))
    error: Exception | None = None
    calls: list[tuple[str, uuid.UUID, Period | None]] = field(default_factory=list)

    async def period_totals(self, user_id: uuid.UUID, period: Period) -> PeriodTotals:
        self.calls.append(("period_totals", user_id, period))
        if self.error is not None:
            raise self.error
        return self.totals

    async def streak(self, user_id: uuid.UUID) -> Streak:
        self.calls.append(("streak", user_id, None))
        if self.error is not None:
            raise self.error
        return self.stored_streak


@dataclass
class FakeDailyRepository:
    """In-memory DailyReadRepository returning the given active days."""

    stored: list[DayMetrics] = field(default_factory=list)
    error: Exception | None = None
    calls: list[tuple[uuid.UUID, Period]] = field(default_factory=list)

    async def active_days(self, user_id: uuid.UUID, period: Period) -> list[DayMetrics]:
        self.calls.append((user_id, period))
        if self.error is not None:
            raise self.error
        return list(self.stored)


@dataclass
class FakeSubjectsRepository:
    """In-memory SubjectsReadRepository returning the given subjects as stored."""

    stored: list[SubjectMetrics] = field(default_factory=list)
    error: Exception | None = None
    calls: list[tuple[uuid.UUID, Period]] = field(default_factory=list)

    async def subjects_in_period(self, user_id: uuid.UUID, period: Period) -> list[SubjectMetrics]:
        self.calls.append((user_id, period))
        if self.error is not None:
            raise self.error
        return list(self.stored)
