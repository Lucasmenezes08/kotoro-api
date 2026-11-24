import uuid
from datetime import date

import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient
from tests.support.fakes import FakeDailyRepository
from tests.support.tokens import make_token

from kotoro_analytics.api.dependencies import get_daily_repository
from kotoro_analytics.api.main import create_app
from kotoro_analytics.application.queries.daily import DayMetrics
from kotoro_analytics.config import Settings

URL = "/api/v1/analytics/daily"
ANA = uuid.UUID("11111111-1111-4111-8111-111111111111")
AROUND_GAP = {"from": "2026-11-12", "to": "2026-11-14"}


@pytest.fixture
def repository() -> FakeDailyRepository:
    return FakeDailyRepository(
        stored=[
            DayMetrics(date(2026, 11, 12), 4500, 2, 1),
            DayMetrics(date(2026, 11, 14), 4500, 2, 1),
        ]
    )


@pytest.fixture
def client(repository: FakeDailyRepository) -> TestClient:
    app: FastAPI = create_app(Settings(environment="test"))
    app.dependency_overrides[get_daily_repository] = lambda: repository
    with TestClient(app, raise_server_exceptions=False) as test_client:
        yield test_client


def _auth(user_id: uuid.UUID = ANA) -> dict[str, str]:
    return {"Authorization": f"Bearer {make_token(user_id)}"}


def test_returns_the_contract_from_adr_0003(client: TestClient) -> None:
    response = client.get(URL, params=AROUND_GAP, headers=_auth())

    assert response.status_code == 200
    assert response.json() == {
        "period": {"from": "2026-11-12", "to": "2026-11-14", "timezone": "America/Sao_Paulo"},
        "days": [
            {
                "date": "2026-11-12",
                "studied_seconds": 4500,
                "completed_cycles": 2,
                "completed_tasks": 1,
            },
            {
                "date": "2026-11-13",
                "studied_seconds": 0,
                "completed_cycles": 0,
                "completed_tasks": 0,
            },
            {
                "date": "2026-11-14",
                "studied_seconds": 4500,
                "completed_cycles": 2,
                "completed_tasks": 1,
            },
        ],
    }


def test_user_comes_from_the_token(client: TestClient, repository: FakeDailyRepository) -> None:
    other = uuid.uuid4()

    client.get(URL, params={**AROUND_GAP, "user_id": str(ANA)}, headers=_auth(other))

    assert [user_id for user_id, _ in repository.calls] == [other]


def test_missing_token_is_401(client: TestClient, repository: FakeDailyRepository) -> None:
    response = client.get(URL, params=AROUND_GAP)

    assert response.status_code == 401
    assert repository.calls == []


def test_too_long_period_is_422(client: TestClient) -> None:
    response = client.get(URL, params={"from": "2025-01-01", "to": "2026-11-30"}, headers=_auth())

    assert response.status_code == 422
    assert response.json()["details"][0]["field"] == "to"


def test_database_down_is_503(client: TestClient, repository: FakeDailyRepository) -> None:
    repository.error = ConnectionRefusedError()

    response = client.get(URL, params=AROUND_GAP, headers=_auth())

    assert response.status_code == 503
    assert response.json() == {"error": "service unavailable"}


def test_openapi_documents_the_endpoint(client: TestClient) -> None:
    operation = client.get("/openapi.json").json()["paths"][URL]["get"]

    assert {p["name"] for p in operation["parameters"]} == {"from", "to"}
    assert set(operation["responses"]) == {"200", "401", "422", "500", "503"}
    assert operation["security"] == [{"KotoroBearer": []}]
