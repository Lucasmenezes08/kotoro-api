import pytest
from sqlalchemy.ext.asyncio import AsyncSession

from kotoro_analytics.config import Settings
from kotoro_analytics.infrastructure.database.session import (
    create_engine,
    create_session_factory,
)


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


def test_engine_uses_configured_pool() -> None:
    engine = create_engine(Settings(db_pool_size=3, db_max_overflow=2))

    assert engine.pool.size() == 3
    assert engine.pool._max_overflow == 2
    assert engine.url.drivername == "postgresql+asyncpg"


@pytest.mark.anyio
async def test_session_factory_creates_independent_sessions() -> None:
    engine = create_engine(Settings())
    factory = create_session_factory(engine)

    async with factory() as first, factory() as second:
        assert isinstance(first, AsyncSession)
        assert first is not second

    await engine.dispose()
