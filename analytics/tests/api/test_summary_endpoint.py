import uuid
from datetime import UTC, date, datetime

import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient
from tests.support.fakes import FakeSummaryRepository
from tests.support.tokens import make_token

from kotoro_analytics.api.dependencies import get_clock, get_summary_repository
from kotoro_analytics.api.main import create_app
from kotoro_analytics.application.queries.summary import PeriodTotals
from kotoro_analytics.config import Settings
from kotoro_analytics.domain.metrics import Streak
from kotoro_analytics.domain.period import Period

URL = "/api/v1/analytics/summary"
ANA = uuid.UUID("11111111-1111-4111-8111-111111111111")
NOVEMBER = {"from": "2026-11-01", "to": "2026-11-30"}
FIXED_NOW = datetime(2026, 11, 20, 15, 0, tzinfo=UTC)


@pytest.fixture
def repository() -> FakeSummaryRepository:
    return FakeSummaryRepository(
        totals=PeriodTotals(75_000, 33, 18, 16),
        stored_streak=Streak(7, 9, date(2026, 11, 20)),
    )


@pytest.fixture
def app(repository: FakeSummaryRepository) -> FastAPI:
    app = create_app(Settings(environment="test"))
    app.dependency_overrides[get_summary_repository] = lambda: repository
    app.dependency_overrides[get_clock] = lambda: lambda: FIXED_NOW
    return app


@pytest.fixture
def client(app: FastAPI) -> TestClient:
    with TestClient(app, raise_server_exceptions=False) as test_client:
        yield test_client


def _auth(user_id: uuid.UUID = ANA) -> dict[str, str]:
    return {"Authorization": f"Bearer {make_token(user_id)}"}


def test_returns_the_contract_from_adr_0002(client: TestClient) -> None:
    response = client.get(URL, params=NOVEMBER, headers=_auth())

    assert response.status_code == 200
    assert response.json() == {
        "period": {"from": "2026-11-01", "to": "2026-11-30", "timezone": "America/Sao_Paulo"},
        "studied_seconds": 75000,
        "completed_cycles": 33,
        "completed_tasks": 18,
        "active_days": 16,
        "streak": {"current_days": 7, "longest_days": 9, "last_active_date": "2026-11-20"},
    }


def test_user_comes_from_the_token(client: TestClient, repository: FakeSummaryRepository) -> None:
    other = uuid.uuid4()

    client.get(URL, params=NOVEMBER, headers=_auth(other))

    assert {user_id for _, user_id, _ in repository.calls} == {other}


def test_query_string_user_id_is_ignored(
    client: TestClient, repository: FakeSummaryRepository
) -> None:
    client.get(URL, params={**NOVEMBER, "user_id": str(uuid.uuid4())}, headers=_auth())

    assert {user_id for _, user_id, _ in repository.calls} == {ANA}


def test_period_is_passed_to_the_repository(
    client: TestClient, repository: FakeSummaryRepository
) -> None:
    client.get(URL, params=NOVEMBER, headers=_auth())

    periods = [period for name, _, period in repository.calls if name == "period_totals"]
    assert periods == [Period(date(2026, 11, 1), date(2026, 11, 30))]


def test_user_without_activity_gets_200_with_zeros(app: FastAPI, client: TestClient) -> None:
    empty = FakeSummaryRepository()
    app.dependency_overrides[get_summary_repository] = lambda: empty

    response = client.get(URL, params=NOVEMBER, headers=_auth())

    assert response.status_code == 200
    body = response.json()
    assert (body["studied_seconds"], body["active_days"]) == (0, 0)
    assert body["streak"] == {"current_days": 0, "longest_days": 0, "last_active_date": None}


def test_missing_token_is_401_even_with_invalid_period(
    client: TestClient, repository: FakeSummaryRepository
) -> None:
    response = client.get(URL, params={"from": "bad"})

    assert response.status_code == 401
    assert response.json() == {"error": "unauthorized"}
    assert repository.calls == []


def test_inverted_period_is_422(client: TestClient, repository: FakeSummaryRepository) -> None:
    response = client.get(URL, params={"from": "2026-11-30", "to": "2026-11-01"}, headers=_auth())

    assert response.status_code == 422
    assert response.json()["details"][0]["field"] == "from"
    assert repository.calls == []


def test_database_down_is_503(client: TestClient, repository: FakeSummaryRepository) -> None:
    repository.error = ConnectionRefusedError("postgresql://u:secret@db")

    response = client.get(URL, params=NOVEMBER, headers=_auth())

    assert response.status_code == 503
    assert response.json() == {"error": "service unavailable"}


def test_openapi_documents_the_endpoint(client: TestClient) -> None:
    operation = client.get("/openapi.json").json()["paths"][URL]["get"]

    assert {p["name"] for p in operation["parameters"]} == {"from", "to"}
    assert set(operation["responses"]) == {"200", "401", "422", "500", "503"}
    assert operation["security"] == [{"KotoroBearer": []}]
