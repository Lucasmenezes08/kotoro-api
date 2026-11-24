"""Performance per subject (ADR 0003, decision 2). The item shape is shared with sprints."""

import uuid
from collections.abc import Iterable
from dataclasses import dataclass
from typing import Protocol
from zoneinfo import ZoneInfo

from kotoro_analytics.domain.period import Period


@dataclass(frozen=True)
class SubjectMetrics:
    subject_id: uuid.UUID
    studied_seconds: int
    completed_cycles: int
    completed_tasks: int


def rank_subjects(subjects: Iterable[SubjectMetrics]) -> list[SubjectMetrics]:
    """Most studied first; ties broken by subject_id so the order is deterministic."""
    return sorted(subjects, key=lambda subject: (-subject.studied_seconds, subject.subject_id.int))


class SubjectsReadRepository(Protocol):
    async def subjects_in_period(self, user_id: uuid.UUID, period: Period) -> list[SubjectMetrics]:
        """One item per subject with activity in `period`, totals summed over its days."""
        ...


@dataclass(frozen=True)
class SubjectsReport:
    period: Period
    timezone: str
    subjects: list[SubjectMetrics]


class GetSubjectsReport:
    def __init__(self, repository: SubjectsReadRepository, *, timezone: ZoneInfo) -> None:
        self._repository = repository
        self._timezone = timezone

    async def execute(self, user_id: uuid.UUID, period: Period) -> SubjectsReport:
        subjects = await self._repository.subjects_in_period(user_id, period)
        return SubjectsReport(
            period=period, timezone=self._timezone.key, subjects=rank_subjects(subjects)
        )
