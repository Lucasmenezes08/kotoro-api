"""Integration fixtures: a throwaway PostgreSQL database migrated with Alembic.

The database is ANALYTICS_TEST_DATABASE_URL, by default `kotoro_analytics_test` on the
local postgres-analytics container (localhost:5433). It is dropped and recreated once
per test session, so development data in `kotoro_analytics` is never touched.
If PostgreSQL is unreachable, every integration test is skipped.
"""

import asyncio
import os
from collections.abc import AsyncIterator, Iterator
from pathlib import Path

import pytest
from alembic import command
from alembic.config import Config
from sqlalchemy import text
from sqlalchemy.engine import make_url
from sqlalchemy.ext.asyncio import AsyncEngine, create_async_engine
from sqlalchemy.pool import NullPool

ANALYTICS_ROOT = Path(__file__).resolve().parents[2]
DEFAULT_TEST_DATABASE_URL = (
    "postgresql+asyncpg://analytics:analytics@localhost:5433/kotoro_analytics_test"
)


def pytest_collection_modifyitems(items: list[pytest.Item]) -> None:
    for item in items:
        if Path(str(item.fspath)).is_relative_to(Path(__file__).parent):
            item.add_marker(pytest.mark.integration)


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


@pytest.fixture(scope="session")
def database_url() -> str:
    return os.environ.get("ANALYTICS_TEST_DATABASE_URL", DEFAULT_TEST_DATABASE_URL)


async def _recreate_database(url: str) -> None:
    target = make_url(url)
    if not target.database or not target.database.endswith("_test"):
        raise RuntimeError(f"refusing to recreate {target.database!r}: name must end in _test")

    admin_url = target.set(database="postgres").render_as_string(hide_password=False)
    engine = create_async_engine(
        admin_url,
        poolclass=NullPool,
        isolation_level="AUTOCOMMIT",
        connect_args={"timeout": 3},
    )
    try:
        async with engine.connect() as connection:
            await connection.execute(
                text(f'DROP DATABASE IF EXISTS "{target.database}" WITH (FORCE)')
            )
            await connection.execute(text(f'CREATE DATABASE "{target.database}"'))
    finally:
        await engine.dispose()


def alembic_config(url: str) -> Config:
    config = Config(str(ANALYTICS_ROOT / "alembic.ini"))
    config.attributes["database_url"] = url
    return config


@pytest.fixture(scope="session")
def migrated_database(database_url: str) -> Iterator[str]:
    """Fresh database at Alembic head. Yields its URL."""
    try:
        asyncio.run(_recreate_database(database_url))
    except (OSError, TimeoutError) as exc:
        pytest.skip(f"PostgreSQL not reachable for integration tests ({type(exc).__name__})")

    command.upgrade(alembic_config(database_url), "head")
    yield database_url


@pytest.fixture
async def writer_engine(migrated_database: str) -> AsyncIterator[AsyncEngine]:
    """Engine with write access (like the future consumer). One per test: own event loop."""
    engine = create_async_engine(migrated_database, poolclass=NullPool)
    try:
        yield engine
    finally:
        await engine.dispose()
