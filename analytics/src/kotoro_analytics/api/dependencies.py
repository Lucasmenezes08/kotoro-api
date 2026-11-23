from collections.abc import AsyncIterator
from typing import Annotated

from fastapi import Depends, Request
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker

from kotoro_analytics.config import Settings
from kotoro_analytics.infrastructure.database.health import database_is_reachable


def get_app_settings(request: Request) -> Settings:
    return request.app.state.settings


SettingsDep = Annotated[Settings, Depends(get_app_settings)]


async def get_session(request: Request) -> AsyncIterator[AsyncSession]:
    """Yield one AsyncSession per request and close it when the request ends."""
    session_factory: async_sessionmaker[AsyncSession] = request.app.state.session_factory
    async with session_factory() as session:
        yield session


SessionDep = Annotated[AsyncSession, Depends(get_session)]


async def check_database(session: SessionDep, settings: SettingsDep) -> bool:
    return await database_is_reachable(session, settings.db_readiness_timeout_seconds)


DatabaseReadyDep = Annotated[bool, Depends(check_database)]
