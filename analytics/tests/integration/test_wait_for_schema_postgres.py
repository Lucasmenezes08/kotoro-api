import asyncio
import time

import pytest
from alembic import command

from kotoro_analytics.infrastructure.database.wait_for_schema import (
    head_revisions,
    wait_for_schema,
)
from tests.integration.conftest import alembic_config


@pytest.mark.anyio
async def test_returns_immediately_when_schema_is_at_head(migrated_database: str) -> None:
    started = time.monotonic()

    assert await wait_for_schema(migrated_database, timeout_seconds=5) is True
    assert time.monotonic() - started < 2


def test_times_out_while_schema_is_behind(migrated_database: str) -> None:
    # Sync test: Alembic commands run their own event loop.
    config = alembic_config(migrated_database)
    command.downgrade(config, "base")
    try:
        ready = asyncio.run(
            wait_for_schema(migrated_database, timeout_seconds=0.5, interval_seconds=0.1)
        )
    finally:
        command.upgrade(config, "head")

    assert ready is False


@pytest.mark.anyio
async def test_times_out_when_database_is_unreachable() -> None:
    unreachable = "postgresql+asyncpg://x:x@127.0.0.1:1/nope_test"

    assert await wait_for_schema(unreachable, timeout_seconds=0.3, interval_seconds=0.1) is False


def test_head_is_the_latest_migration() -> None:
    assert head_revisions() == {"0001"}
