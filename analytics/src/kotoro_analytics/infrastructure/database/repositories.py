"""PostgreSQL implementations of the read ports.

Rules (onboarding §5, §15):
- every query filters by `user_id` first; there is no method without it;
- queries read precomputed projections, never recompute history per request;
- filters match the primary keys/indexes created in migration 0001.
"""

import uuid

from sqlalchemy import func, select
from sqlalchemy.ext.asyncio import AsyncSession

from kotoro_analytics.application.queries.summary import PeriodTotals
from kotoro_analytics.domain.metrics import Streak
from kotoro_analytics.domain.period import Period
from kotoro_analytics.infrastructure.database.models import UserDailyMetric, UserStreak


class PostgresSummaryRepository:
    def __init__(self, session: AsyncSession) -> None:
        self._session = session

    async def period_totals(self, user_id: uuid.UUID, period: Period) -> PeriodTotals:
        # Range scan on pk_user_daily_metrics (user_id, metric_date).
        statement = select(
            func.coalesce(func.sum(UserDailyMetric.studied_seconds), 0),
            func.coalesce(func.sum(UserDailyMetric.completed_cycles), 0),
            func.coalesce(func.sum(UserDailyMetric.completed_tasks), 0),
            func.count().filter(UserDailyMetric.completed_cycles > 0),
        ).where(
            UserDailyMetric.user_id == user_id,
            UserDailyMetric.metric_date.between(period.start, period.end),
        )
        studied, cycles, tasks, active_days = (await self._session.execute(statement)).one()
        return PeriodTotals(
            studied_seconds=int(studied),
            completed_cycles=int(cycles),
            completed_tasks=int(tasks),
            active_days=int(active_days),
        )

    async def streak(self, user_id: uuid.UUID) -> Streak:
        # Primary key lookup on pk_user_streaks (user_id).
        statement = select(
            UserStreak.current_streak_days,
            UserStreak.longest_streak_days,
            UserStreak.last_active_date,
        ).where(UserStreak.user_id == user_id)
        row = (await self._session.execute(statement)).one_or_none()
        if row is None:
            return Streak(current_days=0, longest_days=0, last_active_date=None)
        return Streak(
            current_days=row.current_streak_days,
            longest_days=row.longest_streak_days,
            last_active_date=row.last_active_date,
        )
