"""Async engine and session factory for the analytics read database.

See docs/adr/0001-read-database-decisions.md: one AsyncSession per request,
never shared between concurrent tasks.
"""

from sqlalchemy.ext.asyncio import (
    AsyncEngine,
    AsyncSession,
    async_sessionmaker,
    create_async_engine,
)

from kotoro_analytics.config import Settings


def create_engine(settings: Settings) -> AsyncEngine:
    """Create the async engine. No connection is opened until the first query."""
    statement_timeout_ms = int(settings.db_statement_timeout_seconds * 1000)

    return create_async_engine(
        settings.database_url,
        pool_size=settings.db_pool_size,
        max_overflow=settings.db_max_overflow,
        pool_timeout=settings.db_pool_timeout_seconds,
        pool_pre_ping=True,
        connect_args={
            "timeout": settings.db_connect_timeout_seconds,
            "server_settings": {
                "application_name": settings.app_name,
                "statement_timeout": str(statement_timeout_ms),
                # The API only reads projections; writes belong to the consumer.
                "default_transaction_read_only": "on",
            },
        },
    )


def create_session_factory(engine: AsyncEngine) -> async_sessionmaker[AsyncSession]:
    return async_sessionmaker(engine, expire_on_commit=False, autoflush=False)
