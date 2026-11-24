import uuid
from datetime import date
from zoneinfo import ZoneInfo

import pytest
from tests.support.fakes import FakeSubjectsRepository

from kotoro_analytics.application.queries.subjects import (
    GetSubjectsReport,
    SubjectMetrics,
    rank_subjects,
)
from kotoro_analytics.domain.period import Period

USER = uuid.UUID("11111111-1111-4111-8111-111111111111")
NOVEMBER = Period(date(2026, 11, 1), date(2026, 11, 30))
LOW_ID = uuid.UUID("00000000-0000-4000-8000-000000000001")
HIGH_ID = uuid.UUID("ffffffff-0000-4000-8000-000000000001")


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


def test_most_studied_subject_comes_first() -> None:
    little = SubjectMetrics(uuid.uuid4(), 100, 1, 0)
    a_lot = SubjectMetrics(uuid.uuid4(), 9_000, 3, 2)

    assert rank_subjects([little, a_lot]) == [a_lot, little]


def test_ties_are_broken_by_subject_id() -> None:
    high = SubjectMetrics(HIGH_ID, 1_500, 1, 0)
    low = SubjectMetrics(LOW_ID, 1_500, 1, 0)

    assert rank_subjects([high, low]) == [low, high]
    assert rank_subjects([low, high]) == [low, high]


@pytest.mark.anyio
async def test_report_ranks_whatever_the_repository_order() -> None:
    repository = FakeSubjectsRepository(
        stored=[SubjectMetrics(HIGH_ID, 10, 1, 0), SubjectMetrics(LOW_ID, 20, 1, 0)]
    )
    query = GetSubjectsReport(repository, timezone=ZoneInfo("America/Sao_Paulo"))

    report = await query.execute(USER, NOVEMBER)

    assert [subject.subject_id for subject in report.subjects] == [LOW_ID, HIGH_ID]
    assert (report.period, report.timezone) == (NOVEMBER, "America/Sao_Paulo")


@pytest.mark.anyio
async def test_no_activity_gives_an_empty_list() -> None:
    query = GetSubjectsReport(FakeSubjectsRepository(), timezone=ZoneInfo("America/Sao_Paulo"))

    assert (await query.execute(USER, NOVEMBER)).subjects == []
