import uuid
from datetime import UTC, date, datetime

from fastapi.testclient import TestClient
from tests.support.fakes import FakeSummaryRepository
from tests.support.tokens import make_token

from kotoro_analytics.api.dependencies import get_clock, get_summary_repository
from kotoro_analytics.api.main import create_app
from kotoro_analytics.application.queries.summary import PeriodTotals
from kotoro_analytics.config import Settings
from kotoro_analytics.domain.metrics import Streak

URL = "/api/v1/analytics/summary"
ORIGIN = "http://localhost:5173"


def _make_client() -> TestClient:
    app = create_app(Settings(environment="test"))
    app.dependency_overrides[get_summary_repository] = lambda: FakeSummaryRepository(
        totals=PeriodTotals(3600, 2, 1, 1),
        stored_streak=Streak(1, 3, date(2026, 11, 23)),
    )
    app.dependency_overrides[get_clock] = lambda: lambda: datetime(2026, 11, 23, 15, 0, tzinfo=UTC)
    return TestClient(app, raise_server_exceptions=False)


def test_preflight_allows_local_frontend_origin_and_authorization_header() -> None:
    with _make_client() as client:
        response = client.options(
            URL,
            headers={
                "Origin": ORIGIN,
                "Access-Control-Request-Method": "GET",
                "Access-Control-Request-Headers": "Authorization, Content-Type",
            },
        )

    assert response.status_code == 200
    assert response.headers.get("access-control-allow-origin") == ORIGIN
    allow_headers = response.headers.get("access-control-allow-headers", "").lower()
    assert "authorization" in allow_headers


def test_summary_get_includes_cors_header_for_allowed_origin() -> None:
    token = make_token(uuid.UUID("11111111-1111-4111-8111-111111111111"))
    with _make_client() as client:
        response = client.get(
            URL,
            params={"from": "2026-11-01", "to": "2026-11-30"},
            headers={"Origin": ORIGIN, "Authorization": f"Bearer {token}"},
        )

    assert response.status_code == 200
    assert response.headers.get("access-control-allow-origin") == ORIGIN


def test_disallowed_origin_does_not_receive_cors_header() -> None:
    with _make_client() as client:
        response = client.options(
            URL,
            headers={
                "Origin": "https://evil.example.com",
                "Access-Control-Request-Method": "GET",
            },
        )

    assert response.headers.get("access-control-allow-origin") is None
