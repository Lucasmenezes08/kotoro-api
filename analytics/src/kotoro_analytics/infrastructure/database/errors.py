"""Classifies database failures that mean "try again later" (HTTP 503)."""

import asyncpg.exceptions
from sqlalchemy import exc as sa_exc

# Raised by asyncpg itself; connection failures often reach us unwrapped.
_ASYNCPG_UNAVAILABLE = (
    asyncpg.exceptions.PostgresConnectionError,
    asyncpg.exceptions.CannotConnectNowError,
    asyncpg.exceptions.QueryCanceledError,  # statement_timeout
)


def is_database_unavailable(error: BaseException) -> bool:
    """True when the database is down, unreachable, overloaded or timed out."""
    if isinstance(error, sa_exc.DBAPIError):
        if error.connection_invalidated or isinstance(error, sa_exc.OperationalError):
            return True
        return is_database_unavailable(error.orig) if error.orig is not None else False
    return isinstance(
        error,
        (
            sa_exc.TimeoutError,  # pool exhausted
            OSError,  # refused, DNS failure, socket timeout (TimeoutError is an OSError)
            *_ASYNCPG_UNAVAILABLE,
        ),
    )
