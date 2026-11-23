"""GET /api/v1/analytics/summary end to end: real app, real PostgreSQL, Go-style tokens.

Nothing is faked except the clock, pinned to the seed's last active day so the
current streak is deterministic. Expected numbers come from kotoro_analytics.seeds.
"""

import asyncio
import uuid
from collections.abc import Iterator
from datetime import UTC, date, datetime, timedelta
from zoneinfo import ZoneInfo

import pytest
from fastapi.testclient import TestClient
from sqlalchemy import delete, insert
from sqlalchemy.ext.asyncio import create_async_engine
from sqlalchemy.pool import NullPool

from kotoro_analytics.api.dependencies import get_clock
from kotoro_analytics.api.main import create_app
from kotoro_analytics.config import Settings
from kotoro_analytics.infrastructure.database.models import UserDailyMetric
from kotoro_analytics.seeds import (
    DEFAULT_END_DATE,
    USER_ANA,
    USER_BRUNO,
    USER_CARLA,
    apply_seed,
    build_seed_dataset,
)
from tests.support.tokens import make_token

URL = "/api/v1/analytics/summary"
NOVEMBER = {"from": "2026-11-01", "to": "2026-11-30"}
# 12:00 in Sao Paulo on the seed's last active day.
PINNED_NOW = datetime(2026, 11, 20, 15, 0, tzinfo=UTC)


def _run_with_writer(url: str, action) -> None:
    async def run() -> None:
        engine = create_async_engine(url, poolclass=NullPool)
        try:
            await action(engine)
        finally:
            await engine.dispose()

    asyncio.run(run())


@pytest.fixture
def client(migrated_database: str) -> Iterator[TestClient]:
    """Freshly seeded database (resets any row a previous test added for seed users)."""
    dataset = build_seed_dataset(DEFAULT_END_DATE, ZoneInfo("America/Sao_Paulo"))
    _run_with_writer(migrated_database, lambda engine: apply_seed(engine, dataset))

    app = create_app(Settings(environment="test", database_url=migrated_database))
    app.dependency_overrides[get_clock] = lambda: lambda: PINNED_NOW
    with TestClient(app, raise_server_exceptions=False) as test_client:
        yield test_client


def _get(client: TestClient, user_id: uuid.UUID, params: dict[str, str] = NOVEMBER):
    return client.get(
        URL, params=params, headers={"Authorization": f"Bearer {make_token(user_id)}"}
    )


def test_ana_november_matches_the_seed(client: TestClient) -> None:
    response = _get(client, USER_ANA)

    assert response.status_code == 200
    assert response.json() == {
        "period": {"from": "2026-11-01", "to": "2026-11-30", "timezone": "America/Sao_Paulo"},
        "studied_seconds": 75_000,
        "completed_cycles": 33,
        "completed_tasks": 18,
        "active_days": 16,
        "streak": {"current_days": 7, "longest_days": 9, "last_active_date": "2026-11-20"},
    }


def test_late_night_cycle_counts_for_the_local_day(client: TestClient) -> None:
    body = _get(client, USER_ANA, {"from": "2026-11-20", "to": "2026-11-20"}).json()

    # 09:00 (1500 s) + 14:00 (3000 s) + 22:00 (3000 s, already Nov 21 in UTC).
    assert (body["studied_seconds"], body["completed_cycles"], body["active_days"]) == (7_500, 3, 1)


def test_period_with_no_activity_returns_zeros_but_keeps_streak(client: TestClient) -> None:
    body = _get(client, USER_ANA, {"from": "2026-10-01", "to": "2026-10-31"}).json()

    assert (body["studied_seconds"], body["completed_cycles"], body["active_days"]) == (0, 0, 0)
    # The streak is not affected by the period (ADR 0002).
    assert body["streak"]["longest_days"] == 9


def test_users_only_see_their_own_numbers(client: TestClient) -> None:
    ana = _get(client, USER_ANA).json()
    bruno = _get(client, USER_BRUNO).json()

    assert (bruno["studied_seconds"], bruno["completed_tasks"], bruno["active_days"]) == (
        8_100,
        6,
        3,
    )
    assert bruno["streak"] == {
        "current_days": 3,
        "longest_days": 3,
        "last_active_date": "2026-11-20",
    }
    # Same calendar days, different users: nothing leaks between them.
    assert ana["studied_seconds"] == 75_000


def test_user_id_in_the_query_string_cannot_read_another_user(client: TestClient) -> None:
    response = client.get(
        URL,
        params={**NOVEMBER, "user_id": str(USER_ANA)},
        headers={"Authorization": f"Bearer {make_token(USER_BRUNO)}"},
    )

    assert response.json()["studied_seconds"] == 8_100  # Bruno's, not Ana's


@pytest.mark.parametrize("user_id", [USER_CARLA, uuid.uuid4()], ids=["carla", "never-seen"])
def test_user_without_activity_gets_200_with_zeros(client: TestClient, user_id: uuid.UUID) -> None:
    response = _get(client, user_id)

    assert response.status_code == 200
    body = response.json()
    assert (body["studied_seconds"], body["completed_cycles"], body["completed_tasks"]) == (0, 0, 0)
    assert body["active_days"] == 0
    assert body["streak"] == {"current_days": 0, "longest_days": 0, "last_active_date": None}


def test_endpoint_reflects_a_new_projection_row(client: TestClient, migrated_database: str) -> None:
    # What the consumer will do in stage 5: write a projection, then the API reads it.
    async def add_row(engine) -> None:
        async with engine.begin() as connection:
            await connection.execute(
                delete(UserDailyMetric).where(UserDailyMetric.user_id == USER_CARLA)
            )
            await connection.execute(
                insert(UserDailyMetric),
                [
                    {
                        "user_id": USER_CARLA,
                        "metric_date": date(2026, 11, 15),
                        "studied_seconds": 1_800,
                        "completed_cycles": 1,
                        "completed_tasks": 2,
                    }
                ],
            )

    assert _get(client, USER_CARLA).json()["studied_seconds"] == 0
    _run_with_writer(migrated_database, add_row)

    body = _get(client, USER_CARLA).json()
    assert (body["studied_seconds"], body["completed_tasks"], body["active_days"]) == (1_800, 2, 1)


def test_expired_token_is_401(client: TestClient) -> None:
    token = make_token(USER_ANA, expires_in=timedelta(minutes=-5))

    response = client.get(URL, params=NOVEMBER, headers={"Authorization": f"Bearer {token}"})

    assert response.status_code == 401
    assert response.json() == {"error": "unauthorized"}


@pytest.mark.parametrize(
    "params",
    [
        {"from": "2026-11-30", "to": "2026-11-01"},
        {"from": "2025-01-01", "to": "2026-11-30"},
        {"from": "2026-11-01"},
    ],
    ids=["inverted", "too-long", "missing-to"],
)
def test_invalid_period_is_422(client: TestClient, params: dict[str, str]) -> None:
    response = _get(client, USER_ANA, params)

    assert response.status_code == 422
    assert response.json()["error"] == "invalid request"


def test_database_unreachable_is_503(migrated_database: str) -> None:
    app = create_app(
        Settings(
            environment="test",
            database_url="postgresql+asyncpg://analytics:analytics@127.0.0.1:1/kotoro_analytics_test",
            db_connect_timeout_seconds=1,
        )
    )
    with TestClient(app, raise_server_exceptions=False) as client:
        response = _get(client, USER_ANA)

    assert response.status_code == 503
    assert response.json() == {"error": "service unavailable"}
