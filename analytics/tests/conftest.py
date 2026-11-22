import pytest
from fastapi.testclient import TestClient

from kotoro_analytics.api.main import create_app
from kotoro_analytics.config import Settings


@pytest.fixture
def client() -> TestClient:
    app = create_app(Settings(environment="test"))
    with TestClient(app) as test_client:
        yield test_client
