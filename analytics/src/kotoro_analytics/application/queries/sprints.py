"""Analytical summary of one sprint (ADR 0003, decision 3)."""

import uuid
from dataclasses import dataclass
from datetime import datetime
from typing import Protocol

from kotoro_analytics.application.errors import NotFoundError
from kotoro_analytics.application.queries.subjects import SubjectMetrics, rank_subjects


class SprintNotFoundError(NotFoundError):
    """No analytics for this sprint and user: unknown, without finished cycles, or not theirs."""


@dataclass(frozen=True)
class SprintTotals:
    sprint_id: uuid.UUID
    studied_seconds: int
    completed_cycles: int
    completed_tasks: int
    first_cycle_started_at: datetime | None
    last_cycle_ended_at: datetime | None


class SprintReadRepository(Protocol):
    """Every method takes the user: a sprint id alone never grants access."""

    async def sprint_totals(
        self, user_id: uuid.UUID, sprint_id: uuid.UUID
    ) -> SprintTotals | None: ...

    async def sprint_subjects(
        self, user_id: uuid.UUID, sprint_id: uuid.UUID
    ) -> list[SubjectMetrics]: ...


@dataclass(frozen=True)
class SprintSummary:
    totals: SprintTotals
    subjects: list[SubjectMetrics]


class GetSprintSummary:
    def __init__(self, repository: SprintReadRepository) -> None:
        self._repository = repository

    async def execute(self, user_id: uuid.UUID, sprint_id: uuid.UUID) -> SprintSummary:
        totals = await self._repository.sprint_totals(user_id, sprint_id)
        if totals is None:
            raise SprintNotFoundError
        subjects = await self._repository.sprint_subjects(user_id, sprint_id)
        return SprintSummary(totals=totals, subjects=rank_subjects(subjects))
