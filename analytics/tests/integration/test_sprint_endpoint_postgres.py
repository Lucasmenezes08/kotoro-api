"""GET /api/v1/analytics/sprints/{sprint_id} end to end against the seeded PostgreSQL."""

import asyncio
import uuid
from collections.abc import Iterator
from datetime import date
from zoneinfo import ZoneInfo

import pytest
from fastapi.testclient import TestClient
from sqlalchemy.ext.asyncio import create_async_engine
from sqlalchemy.pool import NullPool

from kotoro_analytics.api.main import create_app
from kotoro_analytics.config import Settings
from kotoro_analytics.seeds import (
    DEFAULT_END_DATE,
    SUBJECT_ANA_MATH,
    SUBJECT_ANA_PORTUGUESE,
    USER_ANA,
    USER_BRUNO,
    USER_CARLA,
    apply_seed,
    build_seed_dataset,
    seed_sprint_id,
)
from tests.support.tokens import make_token

ANA_LAST_SPRINT = seed_sprint_id(USER_ANA, DEFAULT_END_DATE)


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


def _get(client: TestClient, user_id: uuid.UUID, sprint_id: uuid.UUID):
    return client.get(
        f"/api/v1/analytics/sprints/{sprint_id}",
        headers={"Authorization": f"Bearer {make_token(user_id)}"},
    )


def test_example_from_adr_0003(client: TestClient) -> None:
    response = _get(client, USER_ANA, ANA_LAST_SPRINT)

    assert response.status_code == 200
    assert response.json() == {
        "sprint_id": str(ANA_LAST_SPRINT),
        "studied_seconds": 7_500,
        "completed_cycles": 3,
        "completed_tasks": 3,
        "first_cycle_started_at": "2026-11-20T12:00:00Z",
        "last_cycle_ended_at": "2026-11-21T01:50:00Z",
        "subjects": [
            {
                "subject_id": str(SUBJECT_ANA_MATH),
                "studied_seconds": 4_500,
                "completed_cycles": 2,
                "completed_tasks": 3,
            },
            {
                "subject_id": str(SUBJECT_ANA_PORTUGUESE),
                "studied_seconds": 3_000,
                "completed_cycles": 1,
                "completed_tasks": 0,
            },
        ],
    }


def test_subjects_add_up_to_the_sprint_totals(client: TestClient) -> None:
    body = _get(client, USER_ANA, seed_sprint_id(USER_ANA, date(2026, 11, 4))).json()

    assert body["studied_seconds"] == sum(s["studied_seconds"] for s in body["subjects"]) == 4_500
    assert body["completed_cycles"] == sum(s["completed_cycles"] for s in body["subjects"]) == 2


def test_bruno_cannot_see_anas_sprint(client: TestClient) -> None:
    anas = _get(client, USER_BRUNO, ANA_LAST_SPRINT)
    unknown = _get(client, USER_BRUNO, uuid.uuid4())

    assert anas.status_code == unknown.status_code == 404
    assert anas.json() == unknown.json() == {"error": "not found"}


def test_bruno_sees_his_own_sprint(client: TestClient) -> None:
    body = _get(client, USER_BRUNO, seed_sprint_id(USER_BRUNO, DEFAULT_END_DATE)).json()

    assert (body["studied_seconds"], body["completed_cycles"], body["completed_tasks"]) == (
        2_700,
        1,
        2,
    )


def test_day_without_activity_has_no_sprint_analytics(client: TestClient) -> None:
    # Nov 13 is Ana's gap day: no finished cycle, so no projection, so 404.
    assert _get(client, USER_ANA, seed_sprint_id(USER_ANA, date(2026, 11, 13))).status_code == 404


def test_user_without_activity_gets_404(client: TestClient) -> None:
    assert _get(client, USER_CARLA, ANA_LAST_SPRINT).status_code == 404
