"""GET /api/v1/analytics/daily end to end against the seeded PostgreSQL."""

import asyncio
import uuid
from collections.abc import Iterator
from datetime import date, timedelta
from zoneinfo import ZoneInfo

import pytest
from fastapi.testclient import TestClient
from sqlalchemy.ext.asyncio import create_async_engine
from sqlalchemy.pool import NullPool

from kotoro_analytics.api.main import create_app
from kotoro_analytics.config import Settings
from kotoro_analytics.seeds import (
    DEFAULT_END_DATE,
    USER_ANA,
    USER_BRUNO,
    USER_CARLA,
    apply_seed,
    build_seed_dataset,
)
from tests.support.tokens import make_token

URL = "/api/v1/analytics/daily"
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


def _days(client: TestClient, user_id: uuid.UUID, params: dict[str, str] = NOVEMBER) -> list:
    response = client.get(
        URL, params=params, headers={"Authorization": f"Bearer {make_token(user_id)}"}
    )
    assert response.status_code == 200
    return response.json()["days"]


def test_example_from_adr_0003(client: TestClient) -> None:
    assert _days(client, USER_ANA, {"from": "2026-11-12", "to": "2026-11-14"}) == [
        {
            "date": "2026-11-12",
            "studied_seconds": 4500,
            "completed_cycles": 2,
            "completed_tasks": 1,
        },
        {"date": "2026-11-13", "studied_seconds": 0, "completed_cycles": 0, "completed_tasks": 0},
        {
            "date": "2026-11-14",
            "studied_seconds": 4500,
            "completed_cycles": 2,
            "completed_tasks": 1,
        },
    ]


def test_month_has_one_ascending_entry_per_day(client: TestClient) -> None:
    days = _days(client, USER_ANA)

    expected_dates = [str(date(2026, 11, 1) + timedelta(days=offset)) for offset in range(30)]
    assert [day["date"] for day in days] == expected_dates


def test_series_adds_up_to_the_summary(client: TestClient) -> None:
    days = _days(client, USER_ANA)

    assert sum(day["studied_seconds"] for day in days) == 75_000
    assert sum(day["completed_cycles"] for day in days) == 33
    assert sum(1 for day in days if day["completed_cycles"] > 0) == 16


def test_late_night_cycle_is_on_the_local_day(client: TestClient) -> None:
    days = {day["date"]: day for day in _days(client, USER_ANA)}

    assert days["2026-11-20"]["studied_seconds"] == 7_500
    assert days["2026-11-21"]["studied_seconds"] == 0


def test_users_only_see_their_own_days(client: TestClient) -> None:
    bruno = [day for day in _days(client, USER_BRUNO) if day["completed_cycles"] > 0]

    assert [day["date"] for day in bruno] == ["2026-11-18", "2026-11-19", "2026-11-20"]
    assert {day["studied_seconds"] for day in bruno} == {2_700}


def test_user_without_activity_gets_a_zero_series(client: TestClient) -> None:
    days = _days(client, USER_CARLA)

    assert len(days) == 30
    assert {(d["studied_seconds"], d["completed_cycles"], d["completed_tasks"]) for d in days} == {
        (0, 0, 0)
    }
