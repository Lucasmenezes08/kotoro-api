import io
import json
import logging
from collections.abc import Iterator

import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient
from tests.support.tokens import make_token

from kotoro_analytics.api.main import create_app
from kotoro_analytics.config import Settings
from kotoro_analytics.observability.logging import configure_logging, correlation_id

SECRET_IN_ERROR = "pa55w0rd-in-exception"


@pytest.fixture
def log_stream() -> Iterator[io.StringIO]:
    output = io.StringIO()
    configure_logging(
        Settings(
            environment="test",
            database_url=f"postgresql+asyncpg://u:{SECRET_IN_ERROR}@db:5432/x",
        ),
        log_format="json",
        stream=output,
    )
    yield output
    root = logging.getLogger()
    for handler in [h for h in root.handlers if h.get_name() == "kotoro-analytics"]:
        root.removeHandler(handler)
    correlation_id.set(None)


@pytest.fixture
def client(log_stream: io.StringIO) -> Iterator[TestClient]:
    app: FastAPI = create_app(Settings(environment="test"))

    @app.get("/_test/boom")
    def boom() -> None:
        raise RuntimeError(f"postgresql+asyncpg://u:{SECRET_IN_ERROR}@db:5432/x refused")

    with TestClient(app, raise_server_exceptions=False) as test_client:
        yield test_client


def _access_lines(stream: io.StringIO) -> list[dict]:
    lines = [json.loads(line) for line in stream.getvalue().splitlines()]
    return [line for line in lines if line["logger"] == "kotoro_analytics.access"]


def test_generates_a_request_id_when_none_is_sent(client: TestClient) -> None:
    response = client.get("/healthz")

    assert len(response.headers["X-Request-ID"]) == 32


def test_echoes_a_safe_request_id(client: TestClient) -> None:
    response = client.get("/healthz", headers={"X-Request-ID": "frontend-abc.123"})

    assert response.headers["X-Request-ID"] == "frontend-abc.123"


@pytest.mark.parametrize("unsafe", ["has spaces", "x" * 200, "inject\\nline"])
def test_replaces_an_unsafe_request_id(client: TestClient, unsafe: str) -> None:
    response = client.get("/healthz", headers={"X-Request-ID": unsafe})

    assert response.headers["X-Request-ID"] != unsafe


def test_access_line_has_request_id_status_and_duration(
    client: TestClient, log_stream: io.StringIO
) -> None:
    client.get("/healthz", headers={"X-Request-ID": "req-1"})

    [line] = _access_lines(log_stream)
    assert line["correlation_id"] == "req-1"
    assert (line["http_method"], line["http_path"], line["http_status"]) == ("GET", "/healthz", 200)
    assert line["duration_ms"] >= 0


def test_access_line_never_contains_query_string_or_token(
    client: TestClient, log_stream: io.StringIO
) -> None:
    token = make_token("11111111-1111-4111-8111-111111111111")

    client.get(
        "/api/v1/analytics/summary?from=2026-11-30&to=2026-11-01",
        headers={"Authorization": f"Bearer {token}"},
    )

    # Only server-side lines: the test's own HTTP client (httpx) logs the URL it called.
    server_lines = [
        line for line in log_stream.getvalue().splitlines() if json.loads(line)["logger"] != "httpx"
    ]
    assert server_lines
    assert not any(token in line or "from=" in line for line in server_lines)
    assert _access_lines(log_stream)[0]["http_status"] == 422


def test_unhandled_error_keeps_request_id_and_hides_secrets(
    client: TestClient, log_stream: io.StringIO
) -> None:
    response = client.get("/_test/boom", headers={"X-Request-ID": "req-500"})

    assert response.status_code == 500
    assert response.headers["X-Request-ID"] == "req-500"
    assert SECRET_IN_ERROR not in response.text
    output = log_stream.getvalue()
    assert "Traceback" in output  # the traceback is logged...
    assert SECRET_IN_ERROR not in output  # ...without the secret
    assert _access_lines(log_stream)[0]["http_status"] == 500
