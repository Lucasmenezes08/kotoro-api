import logging

import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient
from sqlalchemy import exc as sa_exc

from kotoro_analytics.api.dependencies import PeriodDep
from kotoro_analytics.api.main import create_app
from kotoro_analytics.config import Settings

SECRET = "pa55w0rd-in-error-message"


class _Orig(Exception):
    pass


@pytest.fixture
def client() -> TestClient:
    app: FastAPI = create_app(Settings(environment="test"))

    # Test-only routes: real dashboard routes arrive in later steps.
    @app.get("/_test/period")
    def period(period: PeriodDep) -> dict[str, int]:
        return {"days": period.days}

    @app.get("/_test/bug")
    def bug() -> None:
        raise RuntimeError(f"connection to postgresql://u:{SECRET}@db failed")

    @app.get("/_test/database-down")
    def database_down() -> None:
        raise ConnectionRefusedError(f"postgresql://u:{SECRET}@db")

    @app.get("/_test/bad-sql")
    def bad_sql() -> None:
        raise sa_exc.ProgrammingError(f"SELECT '{SECRET}'", {}, _Orig())

    # raise_server_exceptions=False: inspect the response a real client would get.
    with TestClient(app, raise_server_exceptions=False) as test_client:
        yield test_client


def test_valid_period(client: TestClient) -> None:
    response = client.get("/_test/period", params={"from": "2026-11-01", "to": "2026-11-30"})

    assert response.status_code == 200
    assert response.json() == {"days": 30}


@pytest.mark.parametrize(
    ("params", "field"),
    [
        ({"to": "2026-11-30"}, "from"),
        ({"from": "2026-11-01"}, "to"),
        ({"from": "01/11/2026", "to": "2026-11-30"}, "from"),
        ({"from": "2026-11-01", "to": "2026-02-30"}, "to"),
        ({"from": "2026-11-30", "to": "2026-11-01"}, "from"),
        ({"from": "2025-01-01", "to": "2026-11-30"}, "to"),
    ],
    ids=["missing-from", "missing-to", "bad-format", "impossible-date", "inverted", "too-long"],
)
def test_invalid_period_returns_422(client: TestClient, params: dict, field: str) -> None:
    response = client.get("/_test/period", params=params)

    assert response.status_code == 422
    body = response.json()
    assert body["error"] == "invalid request"
    assert [detail["field"] for detail in body["details"]] == [field]
    assert body["details"][0]["message"]


def test_unexpected_error_returns_generic_500(
    client: TestClient, caplog: pytest.LogCaptureFixture
) -> None:
    with caplog.at_level(logging.ERROR):
        response = client.get("/_test/bug")

    assert response.status_code == 500
    assert response.json() == {"error": "internal server error"}
    assert SECRET not in response.text
    assert "unhandled error on GET /_test/bug" in caplog.text


def test_database_down_returns_503(client: TestClient) -> None:
    response = client.get("/_test/database-down")

    assert response.status_code == 503
    assert response.json() == {"error": "service unavailable"}
    assert SECRET not in response.text


def test_sql_bug_is_500_not_503(client: TestClient) -> None:
    response = client.get("/_test/bad-sql")

    assert response.status_code == 500
    assert response.json() == {"error": "internal server error"}
    assert SECRET not in response.text
