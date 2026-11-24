import uuid
from datetime import UTC, datetime, timedelta, timezone

import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient
from tests.support.fakes import FakeSprintRepository
from tests.support.tokens import make_token

from kotoro_analytics.api.dependencies import get_sprint_repository
from kotoro_analytics.api.main import create_app
from kotoro_analytics.application.queries.sprints import SprintTotals
from kotoro_analytics.application.queries.subjects import SubjectMetrics
from kotoro_analytics.config import Settings

ANA = uuid.UUID("11111111-1111-4111-8111-111111111111")
BRUNO = uuid.UUID("22222222-2222-4222-8222-222222222222")
SPRINT = uuid.UUID("34603048-a584-5fe3-b49c-7f792b0fe040")
MATH = uuid.UUID("aaaaaaaa-0000-4000-8000-000000000001")
PORTUGUESE = uuid.UUID("aaaaaaaa-0000-4000-8000-000000000002")
SAO_PAULO_OFFSET = timezone(timedelta(hours=-3))


def _url(sprint_id: object) -> str:
    return f"/api/v1/analytics/sprints/{sprint_id}"


@pytest.fixture
def repository() -> FakeSprintRepository:
    totals = SprintTotals(
        sprint_id=SPRINT,
        studied_seconds=7_500,
        completed_cycles=3,
        completed_tasks=3,
        # Stored with a -03:00 offset to prove the API always answers in UTC.
        first_cycle_started_at=datetime(2026, 11, 20, 9, 0, tzinfo=SAO_PAULO_OFFSET),
        last_cycle_ended_at=datetime(2026, 11, 21, 1, 50, tzinfo=UTC),
    )
    subjects = [SubjectMetrics(MATH, 4_500, 2, 3), SubjectMetrics(PORTUGUESE, 3_000, 1, 0)]
    return FakeSprintRepository(sprints={(ANA, SPRINT): (totals, subjects)})


@pytest.fixture
def client(repository: FakeSprintRepository) -> TestClient:
    app: FastAPI = create_app(Settings(environment="test"))
    app.dependency_overrides[get_sprint_repository] = lambda: repository
    with TestClient(app, raise_server_exceptions=False) as test_client:
        yield test_client


def _auth(user_id: uuid.UUID = ANA) -> dict[str, str]:
    return {"Authorization": f"Bearer {make_token(user_id)}"}


def test_returns_the_contract_from_adr_0003(client: TestClient) -> None:
    response = client.get(_url(SPRINT), headers=_auth())

    assert response.status_code == 200
    assert response.json() == {
        "sprint_id": str(SPRINT),
        "studied_seconds": 7_500,
        "completed_cycles": 3,
        "completed_tasks": 3,
        "first_cycle_started_at": "2026-11-20T12:00:00Z",
        "last_cycle_ended_at": "2026-11-21T01:50:00Z",
        "subjects": [
            {
                "subject_id": str(MATH),
                "studied_seconds": 4_500,
                "completed_cycles": 2,
                "completed_tasks": 3,
            },
            {
                "subject_id": str(PORTUGUESE),
                "studied_seconds": 3_000,
                "completed_cycles": 1,
                "completed_tasks": 0,
            },
        ],
    }


def test_another_users_sprint_looks_exactly_like_an_unknown_one(client: TestClient) -> None:
    someone_elses = client.get(_url(SPRINT), headers=_auth(BRUNO))
    unknown = client.get(_url(uuid.uuid4()), headers=_auth(BRUNO))

    assert someone_elses.status_code == unknown.status_code == 404
    assert someone_elses.json() == unknown.json() == {"error": "not found"}


def test_user_comes_from_the_token(client: TestClient, repository: FakeSprintRepository) -> None:
    client.get(_url(SPRINT), params={"user_id": str(ANA)}, headers=_auth(BRUNO))

    assert {user_id for _, user_id, _ in repository.calls} == {BRUNO}


def test_malformed_sprint_id_is_422(client: TestClient) -> None:
    response = client.get(_url("not-a-uuid"), headers=_auth())

    assert response.status_code == 422
    assert response.json()["details"][0]["field"] == "sprint_id"


def test_missing_token_is_401(client: TestClient, repository: FakeSprintRepository) -> None:
    assert client.get(_url(SPRINT)).status_code == 401
    assert repository.calls == []


def test_database_down_is_503(client: TestClient, repository: FakeSprintRepository) -> None:
    repository.error = ConnectionRefusedError()

    assert client.get(_url(SPRINT), headers=_auth()).status_code == 503


def test_openapi_documents_the_endpoint(client: TestClient) -> None:
    operation = client.get("/openapi.json").json()["paths"]["/api/v1/analytics/sprints/{sprint_id}"]

    assert set(operation["get"]["responses"]) == {"200", "401", "404", "422", "500", "503"}
    assert operation["get"]["security"] == [{"KotoroBearer": []}]
