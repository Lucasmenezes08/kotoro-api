"""GET /api/v1/analytics/subjects end to end against the seeded PostgreSQL."""

import asyncio
import uuid
from collections.abc import Iterator
from zoneinfo import ZoneInfo

import pytest
from fastapi.testclient import TestClient
from sqlalchemy.ext.asyncio import create_async_engine
from sqlalchemy.pool import NullPool

from kotoro_analytics.api.main import create_app
from kotoro_analytics.config import Settings
from kotoro_analytics.seeds import (
    DEFAULT_END_DATE,
    SUBJECT_ANA_BIOLOGY,
    SUBJECT_ANA_MATH,
    SUBJECT_ANA_PORTUGUESE,
    SUBJECT_BRUNO_PHYSICS,
    USER_ANA,
    USER_BRUNO,
    USER_CARLA,
    apply_seed,
    build_seed_dataset,
)
from tests.support.tokens import make_token

URL = "/api/v1/analytics/subjects"
NOVEMBER = {"from": "2026-11-01", "to": "2026-11-30"}


@pytest.fixture
def client(migrated_database: str) -> Iterator[TestClient]:
    async def seed() -> None:
        engine = create_async_engine(migrated_database, poolclass=NullPool)
        try:
            dataset = build_seed_dataset(DEFAULT_END_DATE, ZoneInfo("America/Sao_Paulo"))
            await apply_seed(engine, dataset)
        finally:
            await engine.dispose()

    asyncio.run(seed())
    app = create_app(Settings(environment="test", database_url=migrated_database))
    with TestClient(app, raise_server_exceptions=False) as test_client:
        yield test_client


def _subjects(
    client: TestClient, user_id: uuid.UUID, params: dict[str, str] = NOVEMBER
) -> list[dict]:
    response = client.get(
        URL, params=params, headers={"Authorization": f"Bearer {make_token(user_id)}"}
    )
    assert response.status_code == 200
    return response.json()["subjects"]


def _row(subject_id: uuid.UUID, seconds: int, cycles: int, tasks: int) -> dict:
    return {
        "subject_id": str(subject_id),
        "studied_seconds": seconds,
        "completed_cycles": cycles,
        "completed_tasks": tasks,
    }


def test_example_from_adr_0003(client: TestClient) -> None:
    assert _subjects(client, USER_ANA) == [
        _row(SUBJECT_ANA_MATH, 27_000, 12, 8),
        _row(SUBJECT_ANA_PORTUGUESE, 25_500, 11, 5),
        _row(SUBJECT_ANA_BIOLOGY, 22_500, 10, 5),
    ]


def test_subjects_add_up_to_the_summary(client: TestClient) -> None:
    subjects = _subjects(client, USER_ANA)

    assert sum(subject["studied_seconds"] for subject in subjects) == 75_000
    assert sum(subject["completed_cycles"] for subject in subjects) == 33
    assert sum(subject["completed_tasks"] for subject in subjects) == 18


def test_period_filters_the_days(client: TestClient) -> None:
    # Nov 20: 09:00 Math (1500 s), 14:00 Portuguese (3000 s), 22:00 Math (3000 s, 2 tasks).
    assert _subjects(client, USER_ANA, {"from": "2026-11-20", "to": "2026-11-20"}) == [
        _row(SUBJECT_ANA_MATH, 4_500, 2, 3),
        _row(SUBJECT_ANA_PORTUGUESE, 3_000, 1, 0),
    ]


def test_users_only_see_their_own_subjects(client: TestClient) -> None:
    assert _subjects(client, USER_BRUNO) == [_row(SUBJECT_BRUNO_PHYSICS, 8_100, 3, 6)]


def test_user_without_activity_gets_an_empty_list(client: TestClient) -> None:
    assert _subjects(client, USER_CARLA) == []


def test_period_without_activity_gets_an_empty_list(client: TestClient) -> None:
    assert _subjects(client, USER_ANA, {"from": "2026-10-01", "to": "2026-10-31"}) == []
