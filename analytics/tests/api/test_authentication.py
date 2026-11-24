import uuid

import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient
from tests.support.tokens import make_token

from kotoro_analytics.api.dependencies import CurrentUserIdDep
from kotoro_analytics.api.main import create_app
from kotoro_analytics.config import Settings

USER = uuid.UUID("11111111-1111-4111-8111-111111111111")
OTHER_SECRET = "another-secret-with-enough-length-0002"


@pytest.fixture
def client() -> TestClient:
    app: FastAPI = create_app(Settings(environment="test"))

    # Test-only route: the dashboard routes arrive in later steps.
    @app.get("/_test/me")
    def me(user_id: CurrentUserIdDep) -> dict[str, str]:
        return {"user_id": str(user_id)}

    with TestClient(app) as test_client:
        yield test_client


def test_valid_token_identifies_the_user(client: TestClient) -> None:
    response = client.get("/_test/me", headers={"Authorization": f"Bearer {make_token(USER)}"})

    assert response.status_code == 200
    assert response.json() == {"user_id": str(USER)}


@pytest.mark.parametrize(
    "headers",
    [
        {},
        {"Authorization": "Bearer"},
        {"Authorization": "Basic dXNlcjpwYXNz"},
        {"Authorization": "Bearer not.a.jwt"},
        {"Authorization": f"Bearer {make_token(USER, secret=OTHER_SECRET)}"},
        {"X-User-Id": str(USER)},
    ],
    ids=["missing", "empty-bearer", "basic-auth", "garbage", "wrong-secret", "user-id-header"],
)
def test_requests_without_a_valid_token_get_401(client: TestClient, headers: dict) -> None:
    response = client.get("/_test/me", headers=headers)

    assert response.status_code == 401
    assert response.json() == {"error": "unauthorized"}
    assert response.headers["WWW-Authenticate"] == "Bearer"


def test_openapi_declares_bearer_security_scheme(client: TestClient) -> None:
    schema = client.get("/openapi.json").json()

    assert schema["components"]["securitySchemes"]["KotoroBearer"]["scheme"] == "bearer"
