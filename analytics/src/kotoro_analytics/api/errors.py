"""HTTP error responses. Bodies follow the Go API shape: {"error": "<message>"} (ADR 0002)."""

from fastapi import FastAPI, Request, status
from fastapi.responses import JSONResponse


class UnauthorizedError(Exception):
    """Raised when the request has no valid bearer token."""


async def _unauthorized_handler(_request: Request, _exc: Exception) -> JSONResponse:
    return JSONResponse(
        status_code=status.HTTP_401_UNAUTHORIZED,
        content={"error": "unauthorized"},
        headers={"WWW-Authenticate": "Bearer"},
    )


def register_error_handlers(app: FastAPI) -> None:
    app.add_exception_handler(UnauthorizedError, _unauthorized_handler)


UNAUTHORIZED_RESPONSE = {
    status.HTTP_401_UNAUTHORIZED: {
        "description": "Missing, invalid or expired bearer token.",
        "content": {"application/json": {"example": {"error": "unauthorized"}}},
    }
}
