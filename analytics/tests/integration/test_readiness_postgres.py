from fastapi.testclient import TestClient

from kotoro_analytics.api.main import create_app
from kotoro_analytics.config import Settings


def test_readyz_is_ready_with_real_database(migrated_database: str) -> None:
    app = create_app(Settings(environment="test", database_url=migrated_database))

    with TestClient(app) as client:
        response = client.get("/readyz")

    assert response.status_code == 200
    assert response.json() == {"status": "ready", "checks": {"database": "ok"}}


def test_readyz_is_unavailable_when_database_refuses_connections(
    migrated_database: str,
) -> None:
    # Port 1 on localhost: nothing listens there, the connection is refused immediately.
    unreachable = "postgresql+asyncpg://analytics:analytics@127.0.0.1:1/kotoro_analytics_test"
    app = create_app(
        Settings(environment="test", database_url=unreachable, db_readiness_timeout_seconds=1)
    )

    with TestClient(app) as client:
        ready = client.get("/readyz")
        alive = client.get("/healthz")

    assert ready.status_code == 503
    assert ready.json()["checks"]["database"] == "unavailable"
    assert alive.status_code == 200
