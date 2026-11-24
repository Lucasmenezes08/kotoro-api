"""HTTP error responses (ADR 0002, decision 3).

Bodies follow the Go API shape, {"error": "<message>"}, plus `details` for validation.
Responses never carry stack traces, SQL, connection strings or exception messages.
"""

import logging
from typing import Any, Literal

import asyncpg.exceptions
from fastapi import FastAPI, Request, status
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from pydantic import BaseModel
from sqlalchemy.exc import SQLAlchemyError

from kotoro_analytics.application.errors import NotFoundError
from kotoro_analytics.domain.period import InvalidPeriodError
from kotoro_analytics.infrastructure.database.errors import is_database_unavailable

logger = logging.getLogger(__name__)

# Query/path prefixes FastAPI puts in `loc`; the field name is what follows.
_LOCATION_PREFIXES = {"query", "path", "header", "body", "cookie"}


class ErrorResponse(BaseModel):
    error: str


class ValidationErrorDetail(BaseModel):
    field: str
    message: str


class ValidationErrorResponse(BaseModel):
    error: Literal["invalid request"]
    details: list[ValidationErrorDetail]


class UnauthorizedError(Exception):
    """Raised when the request has no valid bearer token."""


def _error(status_code: int, message: str, **extra: Any) -> JSONResponse:
    return JSONResponse(status_code=status_code, content={"error": message, **extra})


def _invalid_request(details: list[ValidationErrorDetail]) -> JSONResponse:
    return _error(
        status.HTTP_422_UNPROCESSABLE_CONTENT,
        "invalid request",
        details=[detail.model_dump() for detail in details],
    )


async def _unauthorized_handler(_request: Request, _exc: Exception) -> JSONResponse:
    response = _error(status.HTTP_401_UNAUTHORIZED, "unauthorized")
    response.headers["WWW-Authenticate"] = "Bearer"
    return response


async def _not_found_handler(_request: Request, _exc: Exception) -> JSONResponse:
    return _error(status.HTTP_404_NOT_FOUND, "not found")


async def _request_validation_handler(_request: Request, exc: Exception) -> JSONResponse:
    assert isinstance(exc, RequestValidationError)
    details = []
    for error in exc.errors():
        location = [str(part) for part in error.get("loc", ()) if part not in _LOCATION_PREFIXES]
        details.append(
            # Pydantic's `msg` describes the rule, not the received value.
            ValidationErrorDetail(field=".".join(location) or "request", message=error["msg"])
        )
    return _invalid_request(details)


async def _invalid_period_handler(_request: Request, exc: Exception) -> JSONResponse:
    assert isinstance(exc, InvalidPeriodError)
    return _invalid_request([ValidationErrorDetail(field=exc.field, message=exc.message)])


async def _database_error_handler(request: Request, exc: Exception) -> JSONResponse:
    if is_database_unavailable(exc):
        logger.warning(
            "database unavailable on %s %s: %s",
            request.method,
            request.url.path,
            type(exc).__name__,
        )
        return _error(status.HTTP_503_SERVICE_UNAVAILABLE, "service unavailable")
    return await _unexpected_error_handler(request, exc)


async def _unexpected_error_handler(request: Request, exc: Exception) -> JSONResponse:
    # Full traceback goes to the server log only.
    logger.error("unhandled error on %s %s", request.method, request.url.path, exc_info=exc)
    return _error(status.HTTP_500_INTERNAL_SERVER_ERROR, "internal server error")


def register_error_handlers(app: FastAPI) -> None:
    app.add_exception_handler(UnauthorizedError, _unauthorized_handler)
    app.add_exception_handler(RequestValidationError, _request_validation_handler)
    app.add_exception_handler(InvalidPeriodError, _invalid_period_handler)
    app.add_exception_handler(NotFoundError, _not_found_handler)
    for database_error in (SQLAlchemyError, asyncpg.exceptions.PostgresError, OSError):
        app.add_exception_handler(database_error, _database_error_handler)
    # Last resort. Starlette re-raises after responding so the server can log it too.
    app.add_exception_handler(Exception, _unexpected_error_handler)


# OpenAPI documentation for routes, reused by every dashboard endpoint.
UNAUTHORIZED_RESPONSE: dict[int | str, dict[str, Any]] = {
    status.HTTP_401_UNAUTHORIZED: {
        "model": ErrorResponse,
        "description": "Missing, invalid or expired bearer token.",
        "content": {"application/json": {"example": {"error": "unauthorized"}}},
    }
}

ERROR_RESPONSES: dict[int | str, dict[str, Any]] = {
    **UNAUTHORIZED_RESPONSE,
    status.HTTP_422_UNPROCESSABLE_CONTENT: {
        "model": ValidationErrorResponse,
        "description": "Invalid query parameters.",
        "content": {
            "application/json": {
                "example": {
                    "error": "invalid request",
                    "details": [{"field": "from", "message": "must be on or before 'to'"}],
                }
            }
        },
    },
    status.HTTP_500_INTERNAL_SERVER_ERROR: {
        "model": ErrorResponse,
        "description": "Unexpected failure. Details are only in the server log.",
        "content": {"application/json": {"example": {"error": "internal server error"}}},
    },
    status.HTTP_503_SERVICE_UNAVAILABLE: {
        "model": ErrorResponse,
        "description": "The analytics database is unavailable; retry later.",
        "content": {"application/json": {"example": {"error": "service unavailable"}}},
    },
}

NOT_FOUND_RESPONSE: dict[int | str, dict[str, Any]] = {
    status.HTTP_404_NOT_FOUND: {
        "model": ErrorResponse,
        "description": "Nothing to show for this caller: unknown, no data yet, or not theirs.",
        "content": {"application/json": {"example": {"error": "not found"}}},
    }
}
