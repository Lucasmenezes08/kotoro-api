"""Request id + access log, as a pure ASGI middleware.

- Accepts the caller's `X-Request-ID` when it is a safe token, otherwise generates one.
- Exposes it to every log line (correlation_id) and echoes it in the response header.
- Writes one access line per request: method, path, status, duration. The query string
  is not logged, and headers (Authorization) never are.
"""

import logging
import re
import time
import uuid

from starlette.datastructures import MutableHeaders
from starlette.types import ASGIApp, Message, Receive, Scope, Send

from kotoro_analytics.observability.logging import correlation_id

REQUEST_ID_HEADER = "X-Request-ID"
_SAFE_REQUEST_ID = re.compile(r"^[A-Za-z0-9._-]{1,128}$")

access_logger = logging.getLogger("kotoro_analytics.access")


def _incoming_request_id(scope: Scope) -> str | None:
    for name, value in scope.get("headers", []):
        if name == b"x-request-id":
            candidate = value.decode("latin-1")
            return candidate if _SAFE_REQUEST_ID.match(candidate) else None
    return None


class RequestIdMiddleware:
    def __init__(self, app: ASGIApp) -> None:
        self.app = app

    async def __call__(self, scope: Scope, receive: Receive, send: Send) -> None:
        if scope["type"] != "http":
            await self.app(scope, receive, send)
            return

        request_id = _incoming_request_id(scope) or uuid.uuid4().hex
        # Not reset on exit: each request runs in its own task/context, and the 500
        # handler (outside this middleware) still needs the id.
        correlation_id.set(request_id)
        started = time.perf_counter()
        status_code = 500

        async def send_with_request_id(message: Message) -> None:
            nonlocal status_code
            if message["type"] == "http.response.start":
                status_code = message["status"]
                MutableHeaders(scope=message)[REQUEST_ID_HEADER] = request_id
            await send(message)

        try:
            await self.app(scope, receive, send_with_request_id)
        finally:
            access_logger.info(
                "%s %s %s",
                scope["method"],
                scope["path"],
                status_code,
                extra={
                    "http_method": scope["method"],
                    "http_path": scope["path"],
                    "http_status": status_code,
                    "duration_ms": round((time.perf_counter() - started) * 1000, 2),
                },
            )
