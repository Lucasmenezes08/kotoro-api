"""Structured logging with correlation ids and secret redaction.

Every record becomes one JSON line (or a readable text line for local debugging):

    {"timestamp": "...", "level": "INFO", "logger": "...", "message": "...",
     "correlation_id": "..."}

Redaction runs on the final formatted line, so it also covers exception messages and
tracebacks: configured secrets, credentials inside URLs and bearer tokens never reach
the log output.
"""

import contextvars
import json
import logging
import re
import sys
from collections.abc import Iterable
from datetime import UTC, datetime
from typing import Literal, TextIO

from sqlalchemy.engine import make_url

from kotoro_analytics.config import Settings

# HTTP request id in the API, event id in the consumer.
correlation_id: contextvars.ContextVar[str | None] = contextvars.ContextVar(
    "correlation_id", default=None
)

REDACTED = "***"
_URL_CREDENTIALS = re.compile(r"(?P<scheme>[a-zA-Z][a-zA-Z0-9+.-]*://)(?P<user>[^:/@\s]+):[^@\s]+@")
# Token-like values only (>= 16 chars), so prose such as "missing bearer token" survives.
_BEARER = re.compile(r"(?i)\bbearer\s+[A-Za-z0-9\-._~+/]{16,}=*")
_HANDLER_NAME = "kotoro-analytics"
# Shorter values are too likely to be ordinary words (the local default password is
# "analytics"); they are still masked inside URLs by _URL_CREDENTIALS. Production
# secrets are longer (ANALYTICS_JWT_SECRET must have at least 32 characters).
MIN_SECRET_LENGTH = 12

# Fields every LogRecord has; anything else passed via `extra=` is emitted as well.
_STANDARD_ATTRIBUTES = set(vars(logging.makeLogRecord({}))) | {
    "message",
    "asctime",
    "taskName",
    "color_message",  # uvicorn's ANSI-coloured duplicate of `message`
}


class Redactor:
    def __init__(self, secrets: Iterable[str]) -> None:
        # Longest first so a secret containing another is fully replaced.
        self._secrets = sorted(
            {s for s in secrets if s and len(s) >= MIN_SECRET_LENGTH}, key=len, reverse=True
        )

    def __call__(self, text: str) -> str:
        for secret in self._secrets:
            text = text.replace(secret, REDACTED)
        text = _URL_CREDENTIALS.sub(rf"\g<scheme>\g<user>:{REDACTED}@", text)
        return _BEARER.sub(f"Bearer {REDACTED}", text)


def secrets_from(settings: Settings) -> list[str]:
    secrets = [settings.jwt_signing_secret()]
    for url in (settings.database_url, settings.rabbitmq_url):
        try:
            password = make_url(url).password
        except Exception:  # noqa: BLE001 - a malformed URL just contributes nothing
            password = None
        if password:
            secrets.append(str(password))
    return secrets


class JsonFormatter(logging.Formatter):
    def __init__(self, redactor: Redactor) -> None:
        super().__init__()
        self._redact = redactor

    def format(self, record: logging.LogRecord) -> str:
        entry: dict[str, object] = {
            "timestamp": datetime.fromtimestamp(record.created, UTC).isoformat(),
            "level": record.levelname,
            "logger": record.name,
            "message": record.getMessage(),
            "correlation_id": correlation_id.get(),
        }
        for key, value in vars(record).items():
            if key not in _STANDARD_ATTRIBUTES and not key.startswith("_"):
                entry[key] = value
        if record.exc_info:
            entry["exception"] = self.formatException(record.exc_info)
        return self._redact(json.dumps(entry, default=str, ensure_ascii=False))


class TextFormatter(logging.Formatter):
    def __init__(self, redactor: Redactor) -> None:
        super().__init__("%(asctime)s %(levelname)s %(name)s [%(correlation)s] %(message)s")
        self._redact = redactor

    def format(self, record: logging.LogRecord) -> str:
        record.correlation = correlation_id.get() or "-"
        return self._redact(super().format(record))


def configure_logging(
    settings: Settings,
    *,
    log_format: Literal["json", "text"] | None = None,
    stream: TextIO | None = None,
) -> None:
    """Install the redacting formatter on the root logger. Safe to call more than once."""
    redactor = Redactor(secrets_from(settings))
    chosen = log_format or settings.log_format
    formatter = JsonFormatter(redactor) if chosen == "json" else TextFormatter(redactor)

    handler = logging.StreamHandler(stream or sys.stdout)
    handler.set_name(_HANDLER_NAME)
    handler.setFormatter(formatter)

    root = logging.getLogger()
    for existing in [h for h in root.handlers if h.get_name() == _HANDLER_NAME]:
        root.removeHandler(existing)
    root.addHandler(handler)
    root.setLevel(settings.log_level.upper())

    # Route uvicorn's own loggers through the same handler (no plain-text duplicates).
    for name in ("uvicorn", "uvicorn.error"):
        uvicorn_logger = logging.getLogger(name)
        uvicorn_logger.handlers.clear()
        uvicorn_logger.propagate = True
    # RequestIdMiddleware writes the access log; uvicorn's would include query strings.
    logging.getLogger("uvicorn.access").disabled = True
