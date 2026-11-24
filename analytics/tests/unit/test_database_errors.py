import socket

import asyncpg.exceptions
import pytest
from sqlalchemy import exc as sa_exc

from kotoro_analytics.infrastructure.database.errors import is_database_unavailable


class _Orig(Exception):
    pass


@pytest.mark.parametrize(
    "error",
    [
        ConnectionRefusedError(),
        socket.gaierror(-2, "Name or service not known"),
        TimeoutError(),
        asyncpg.exceptions.CannotConnectNowError("starting up"),
        asyncpg.exceptions.ConnectionDoesNotExistError("closed"),
        asyncpg.exceptions.QueryCanceledError("statement timeout"),
        sa_exc.TimeoutError("pool exhausted"),
        sa_exc.OperationalError("SELECT 1", {}, _Orig()),
        sa_exc.DBAPIError("SELECT 1", {}, _Orig(), connection_invalidated=True),
        sa_exc.DBAPIError("SELECT 1", {}, ConnectionRefusedError()),
    ],
    ids=lambda e: type(e).__name__,
)
def test_unavailable_database_errors(error: BaseException) -> None:
    assert is_database_unavailable(error) is True


@pytest.mark.parametrize(
    "error",
    [
        sa_exc.ProgrammingError("SELECT nope", {}, _Orig()),
        sa_exc.DBAPIError("SELECT 1", {}, _Orig()),
        asyncpg.exceptions.UndefinedTableError("no such table"),
        ValueError("bug"),
    ],
    ids=lambda e: type(e).__name__,
)
def test_bugs_are_not_reported_as_unavailable(error: BaseException) -> None:
    assert is_database_unavailable(error) is False
