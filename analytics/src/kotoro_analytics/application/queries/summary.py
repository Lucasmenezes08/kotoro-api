"""Dashboard summary query (ADR 0002, decision 2).

This module defines what the summary needs from storage. It knows nothing about
HTTP or SQL: the router calls it, and an infrastructure repository implements the port.
"""

import uuid
from dataclasses import dataclass
from typing import Protocol

from kotoro_analytics.domain.metrics import Streak
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
