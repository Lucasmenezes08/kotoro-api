import uuid
from datetime import UTC, datetime

import pytest
from tests.support.fakes import FakeSprintRepository

from kotoro_analytics.application.errors import NotFoundError
from kotoro_analytics.application.queries.sprints import (
    GetSprintSummary,
    SprintNotFoundError,
    SprintTotals,
)
from kotoro_analytics.application.queries.subjects import SubjectMetrics

ANA = uuid.UUID("11111111-1111-4111-8111-111111111111")
BRUNO = uuid.UUID("22222222-2222-4222-8222-222222222222")
SPRINT = uuid.UUID("34603048-a584-5fe3-b49c-7f792b0fe040")
MATH = uuid.UUID("aaaaaaaa-0000-4000-8000-000000000001")
PORTUGUESE = uuid.UUID("aaaaaaaa-0000-4000-8000-000000000002")
TOTALS = SprintTotals(
    sprint_id=SPRINT,
    studied_seconds=7_500,
    completed_cycles=3,
    completed_tasks=3,
    first_cycle_started_at=datetime(2026, 11, 20, 12, 0, tzinfo=UTC),
    last_cycle_ended_at=datetime(2026, 11, 21, 1, 50, tzinfo=UTC),
)


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


@pytest.fixture
def repository() -> FakeSprintRepository:
    # Subjects stored least studied first, to prove the query ranks them.
    subjects = [SubjectMetrics(PORTUGUESE, 3_000, 1, 0), SubjectMetrics(MATH, 4_500, 2, 3)]
    return FakeSprintRepository(sprints={(ANA, SPRINT): (TOTALS, subjects)})


@pytest.mark.anyio
async def test_returns_totals_and_ranked_subjects(repository: FakeSprintRepository) -> None:
    summary = await GetSprintSummary(repository).execute(ANA, SPRINT)

    assert summary.totals == TOTALS
    assert [subject.subject_id for subject in summary.subjects] == [MATH, PORTUGUESE]


@pytest.mark.anyio
async def test_unknown_sprint_is_not_found(repository: FakeSprintRepository) -> None:
    with pytest.raises(SprintNotFoundError):
        await GetSprintSummary(repository).execute(ANA, uuid.uuid4())


@pytest.mark.anyio
async def test_another_users_sprint_is_not_found(repository: FakeSprintRepository) -> None:
    with pytest.raises(NotFoundError):
        await GetSprintSummary(repository).execute(BRUNO, SPRINT)


@pytest.mark.anyio
async def test_subjects_are_not_queried_when_not_found(repository: FakeSprintRepository) -> None:
    with pytest.raises(SprintNotFoundError):
        await GetSprintSummary(repository).execute(BRUNO, SPRINT)

    assert [name for name, _, _ in repository.calls] == ["sprint_totals"]
