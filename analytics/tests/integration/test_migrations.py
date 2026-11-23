import asyncio

from alembic import command
from sqlalchemy import inspect
from sqlalchemy.ext.asyncio import create_async_engine
from sqlalchemy.pool import NullPool

from tests.integration.conftest import alembic_config

READ_MODEL_TABLES = {
    "processed_events",
    "study_cycle_facts",
    "user_daily_metrics",
    "user_subject_metrics",
    "sprint_summaries",
    "user_streaks",
}


def _tables(url: str) -> set[str]:
    async def load() -> set[str]:
        engine = create_async_engine(url, poolclass=NullPool)
        try:
            async with engine.connect() as connection:
                return set(await connection.run_sync(lambda c: inspect(c).get_table_names()))
        finally:
            await engine.dispose()

    return asyncio.run(load())


def _indexes(url: str, table: str) -> set[str]:
    async def load() -> set[str]:
        engine = create_async_engine(url, poolclass=NullPool)
        try:
            async with engine.connect() as connection:
                indexes = await connection.run_sync(lambda c: inspect(c).get_indexes(table))
                return {index["name"] for index in indexes}
        finally:
            await engine.dispose()

    return asyncio.run(load())


def test_upgrade_creates_read_model(migrated_database: str) -> None:
    assert _tables(migrated_database) >= READ_MODEL_TABLES


def test_upgrade_creates_query_indexes(migrated_database: str) -> None:
    assert "ix_study_cycle_facts_user_id_metric_date" in _indexes(
        migrated_database, "study_cycle_facts"
    )
    assert "ix_sprint_summaries_user_id" in _indexes(migrated_database, "sprint_summaries")


def test_models_and_migrations_do_not_drift(migrated_database: str) -> None:
    # Raises if `alembic revision --autogenerate` would produce operations.
    command.check(alembic_config(migrated_database))


def test_downgrade_removes_read_model_and_upgrade_restores_it(migrated_database: str) -> None:
    config = alembic_config(migrated_database)

    command.downgrade(config, "base")
    assert _tables(migrated_database) == {"alembic_version"}

    command.upgrade(config, "head")
    assert _tables(migrated_database) >= READ_MODEL_TABLES
