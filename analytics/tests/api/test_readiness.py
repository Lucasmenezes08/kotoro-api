from collections.abc import Iterator
from contextlib import contextmanager

import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient

from kotoro_analytics.api.dependencies import check_database
from kotoro_analytics.api.main import create_app
from kotoro_analytics.config import Settings


@pytest.fixture
def app() -> FastAPI:
    return create_app(Settings(environment="test"))


@contextmanager
def client_with_database(app: FastAPI, *, reachable: bool) -> Iterator[TestClient]:
    async def fake_check() -> bool:
        return reachable

    app.dependency_overrides[check_database] = fake_check
    try:
        with TestClient(app) as client:
            yield client
    finally:
        app.dependency_overrides.clear()


def test_readyz_returns_200_when_database_is_reachable(app: FastAPI) -> None:
    with client_with_database(app, reachable=True) as client:
        response = client.get("/readyz")

    assert response.status_code == 200
    assert response.json() == {"status": "ready", "checks": {"database": "ok"}}


def test_readyz_returns_503_when_database_is_unreachable(app: FastAPI) -> None:
    with client_with_database(app, reachable=False) as client:
        response = client.get("/readyz")

    assert response.status_code == 503
    assert response.json() == {"status": "unavailable", "checks": {"database": "unavailable"}}


def test_healthz_stays_ok_when_database_is_unreachable(app: FastAPI) -> None:
    with client_with_database(app, reachable=False) as client:
        response = client.get("/healthz")

    assert response.status_code == 200
