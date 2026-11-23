import uuid

import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient
from tests.support.fakes import FakeSubjectsRepository
from tests.support.tokens import make_token

from kotoro_analytics.api.dependencies import get_subjects_repository
from kotoro_analytics.api.main import create_app
from kotoro_analytics.application.queries.subjects import SubjectMetrics
from kotoro_analytics.config import Settings

URL = "/api/v1/analytics/subjects"
ANA = uuid.UUID("11111111-1111-4111-8111-111111111111")
MATH = uuid.UUID("aaaaaaaa-0000-4000-8000-000000000001")
PORTUGUESE = uuid.UUID("aaaaaaaa-0000-4000-8000-000000000002")
NOVEMBER = {"from": "2026-11-01", "to": "2026-11-30"}


@pytest.fixture
def repository() -> FakeSubjectsRepository:
    # Stored least studied first, to prove the API ranks them.
    return FakeSubjectsRepository(
        stored=[
            SubjectMetrics(PORTUGUESE, 25_500, 11, 5),
            SubjectMetrics(MATH, 27_000, 12, 8),
        ]
    )


@pytest.fixture
def client(repository: FakeSubjectsRepository) -> TestClient:
    app: FastAPI = create_app(Settings(environment="test"))
    app.dependency_overrides[get_subjects_repository] = lambda: repository
    with TestClient(app, raise_server_exceptions=False) as test_client:
        yield test_client


def _auth(user_id: uuid.UUID = ANA) -> dict[str, str]:
    return {"Authorization": f"Bearer {make_token(user_id)}"}


def test_returns_the_contract_from_adr_0003(client: TestClient) -> None:
    response = client.get(URL, params=NOVEMBER, headers=_auth())

    assert response.status_code == 200
    assert response.json() == {
        "period": {"from": "2026-11-01", "to": "2026-11-30", "timezone": "America/Sao_Paulo"},
        "subjects": [
            {
                "subject_id": str(MATH),
                "studied_seconds": 27_000,
                "completed_cycles": 12,
                "completed_tasks": 8,
            },
            {
                "subject_id": str(PORTUGUESE),
                "studied_seconds": 25_500,
                "completed_cycles": 11,
                "completed_tasks": 5,
            },
        ],
    }


def test_empty_period_returns_an_empty_list(client: TestClient) -> None:
    client.app.dependency_overrides[get_subjects_repository] = lambda: FakeSubjectsRepository()

    response = client.get(URL, params=NOVEMBER, headers=_auth())

    assert response.status_code == 200
    assert response.json()["subjects"] == []


def test_user_comes_from_the_token(client: TestClient, repository: FakeSubjectsRepository) -> None:
    other = uuid.uuid4()

    client.get(URL, params={**NOVEMBER, "user_id": str(ANA)}, headers=_auth(other))

    assert [user_id for user_id, _ in repository.calls] == [other]


def test_missing_token_is_401(client: TestClient, repository: FakeSubjectsRepository) -> None:
    assert client.get(URL, params=NOVEMBER).status_code == 401
    assert repository.calls == []


def test_inverted_period_is_422(client: TestClient) -> None:
    response = client.get(URL, params={"from": "2026-11-30", "to": "2026-11-01"}, headers=_auth())

    assert response.status_code == 422


def test_database_down_is_503(client: TestClient, repository: FakeSubjectsRepository) -> None:
    repository.error = TimeoutError()

    assert client.get(URL, params=NOVEMBER, headers=_auth()).status_code == 503


def test_openapi_documents_the_endpoint(client: TestClient) -> None:
    operation = client.get("/openapi.json").json()["paths"][URL]["get"]

    assert {p["name"] for p in operation["parameters"]} == {"from", "to"}
    assert set(operation["responses"]) == {"200", "401", "422", "500", "503"}
    assert operation["security"] == [{"KotoroBearer": []}]
