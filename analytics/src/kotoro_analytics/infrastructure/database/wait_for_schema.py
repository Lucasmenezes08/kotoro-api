"""Block until the analytics database is migrated to the latest Alembic revision.

Used as an initContainer in Kubernetes, where nothing orders the migrations Job before
the API and the consumer. Without it, a consumer started on an empty schema would fail
on its first event (missing table), reject it after one retry and lose it.

    python -m kotoro_analytics.infrastructure.database.wait_for_schema --timeout 300

Exit code 0 when the schema is at head, 1 on timeout.
"""

import argparse
import asyncio
import logging
import sys
from pathlib import Path

from alembic.config import Config
from alembic.script import ScriptDirectory
from sqlalchemy import text
from sqlalchemy.ext.asyncio import create_async_engine
from sqlalchemy.pool import NullPool

from kotoro_analytics.config import Settings
from kotoro_analytics.observability.logging import configure_logging

logger = logging.getLogger(__name__)


def _find_alembic_ini() -> Path:
    """alembic.ini sits at the analytics project root (WORKDIR /app in the image)."""
    for candidate in (
        Path.cwd() / "alembic.ini",
        Path(__file__).resolve().parents[4] / "alembic.ini",
    ):
        if candidate.is_file():
            return candidate
    raise FileNotFoundError("alembic.ini not found; run from the analytics project root")


DEFAULT_ALEMBIC_INI = _find_alembic_ini()


def head_revisions(alembic_ini: Path = DEFAULT_ALEMBIC_INI) -> set[str]:
    return set(ScriptDirectory.from_config(Config(str(alembic_ini))).get_heads())


async def current_revisions(database_url: str) -> set[str]:
    """Revisions applied to the database; empty when unreachable or not migrated yet."""
    engine = create_async_engine(database_url, poolclass=NullPool, connect_args={"timeout": 5})
    try:
        async with engine.connect() as connection:
            rows = await connection.execute(text("SELECT version_num FROM alembic_version"))
            return {row[0] for row in rows}
    except Exception as exc:  # noqa: BLE001 - any failure means "not ready yet"
        logger.info("schema not ready yet: %s", type(exc).__name__)
        return set()
    finally:
        await engine.dispose()


async def wait_for_schema(
    database_url: str,
    *,
    timeout_seconds: float,
    interval_seconds: float = 2.0,
    alembic_ini: Path = DEFAULT_ALEMBIC_INI,
) -> bool:
    expected = head_revisions(alembic_ini)
    deadline = asyncio.get_running_loop().time() + timeout_seconds
    while True:
        current = await current_revisions(database_url)
        if current == expected:
            logger.info("schema is at head %s", ", ".join(sorted(expected)))
            return True
        if asyncio.get_running_loop().time() >= deadline:
            logger.error("schema still at %s, expected %s", sorted(current), sorted(expected))
            return False
        await asyncio.sleep(interval_seconds)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--timeout", type=float, default=300.0, help="seconds (default 300)")
    args = parser.parse_args()

    settings = Settings()
    configure_logging(settings)
    ready = asyncio.run(wait_for_schema(settings.database_url, timeout_seconds=args.timeout))
    sys.exit(0 if ready else 1)


if __name__ == "__main__":
    main()
