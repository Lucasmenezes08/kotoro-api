import io
import json
import logging
from collections.abc import Iterator

import pytest

from kotoro_analytics.config import Settings
from kotoro_analytics.observability.logging import (
    REDACTED,
    Redactor,
    configure_logging,
    correlation_id,
)

DB_PASSWORD = "db-pa55w0rd-xyz"
JWT_SECRET = "jwt-secret-value-that-is-long-enough-0001"
RABBIT_PASSWORD = "rabbit-pa55-xyz"


@pytest.fixture
def stream() -> Iterator[io.StringIO]:
    output = io.StringIO()
    settings = Settings(
        environment="test",
        database_url=f"postgresql+asyncpg://analytics:{DB_PASSWORD}@db:5432/x",
        rabbitmq_url=f"amqp://guest:{RABBIT_PASSWORD}@rabbitmq:5672/",
        jwt_secret=JWT_SECRET,
    )
    configure_logging(settings, log_format="json", stream=output)
    yield output
    root = logging.getLogger()
    for handler in [h for h in root.handlers if h.get_name() == "kotoro-analytics"]:
        root.removeHandler(handler)
    correlation_id.set(None)


def _lines(stream: io.StringIO) -> list[dict]:
    return [json.loads(line) for line in stream.getvalue().splitlines()]


def test_each_record_is_one_json_line_with_correlation_id(stream: io.StringIO) -> None:
    correlation_id.set("req-123")

    logging.getLogger("kotoro.test").info("hello %s", "world", extra={"http_status": 200})

    [line] = _lines(stream)
    assert line["message"] == "hello world"
    assert line["level"] == "INFO"
    assert line["logger"] == "kotoro.test"
    assert line["correlation_id"] == "req-123"
    assert line["http_status"] == 200
    assert line["timestamp"].endswith("+00:00")


def test_configured_secrets_never_reach_the_output(stream: io.StringIO) -> None:
    logger = logging.getLogger("kotoro.test")

    logger.warning("password is %s and jwt is %s", DB_PASSWORD, JWT_SECRET)
    try:
        raise RuntimeError(f"cannot connect with {RABBIT_PASSWORD}")
    except RuntimeError:
        logger.exception("failure")

    output = stream.getvalue()
    for secret in (DB_PASSWORD, JWT_SECRET, RABBIT_PASSWORD):
        assert secret not in output
    assert REDACTED in output
    assert "Traceback" in _lines(stream)[1]["exception"]


def test_configuring_twice_does_not_duplicate_lines(stream: io.StringIO) -> None:
    configure_logging(Settings(environment="test"), log_format="json", stream=stream)

    logging.getLogger("kotoro.test").info("once")

    assert stream.getvalue().count('"once"') == 1


@pytest.mark.parametrize(
    ("text", "leaked"),
    [
        ("postgresql+asyncpg://user:s3cret@db:5432/x", "s3cret"),
        ("amqp://guest:hunter2@rabbitmq:5672/", "hunter2"),
        ("Authorization: Bearer eyJhbGciOi.eyJzdWIiOi.c2lnbmF0dXJl", "eyJhbGciOi"),
    ],
    ids=["database-url", "amqp-url", "bearer-token"],
)
def test_redactor_removes_credentials_it_was_not_told_about(text: str, leaked: str) -> None:
    assert leaked not in Redactor([])(text)


def test_redactor_keeps_ordinary_text() -> None:
    assert Redactor(["topsecret"])("GET /api/v1/analytics/summary 200") == (
        "GET /api/v1/analytics/summary 200"
    )


def test_short_common_values_are_not_treated_as_secrets() -> None:
    # The local default database password is "analytics": the word must stay readable.
    redact = Redactor(["analytics", "guest"])

    assert redact("kotoro_analytics.access") == "kotoro_analytics.access"
    # ...but a password inside a URL is masked whatever its length.
    assert "analytics@" not in redact("postgresql://analytics:analytics@db/x")


def test_prose_mentioning_bearer_is_kept() -> None:
    assert Redactor([])("request rejected: missing bearer token") == (
        "request rejected: missing bearer token"
    )


def test_uvicorn_access_logger_is_disabled(stream: io.StringIO) -> None:
    logging.getLogger("uvicorn.access").info('"GET /summary?from=2026-11-01 HTTP/1.1" 200')

    assert "from=" not in stream.getvalue()


def test_uvicorn_color_message_is_not_emitted(stream: io.StringIO) -> None:
    logging.getLogger("uvicorn.error").info(
        "Started server", extra={"color_message": "[36mStarted[0m"}
    )

    [line] = _lines(stream)
    assert "color_message" not in line
