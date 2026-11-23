import logging
import uuid
from collections.abc import AsyncIterator
from typing import Annotated

from fastapi import Depends, Request, Security
from fastapi.security import HTTPAuthorizationCredentials, HTTPBearer
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker

from kotoro_analytics.api.errors import UnauthorizedError
from kotoro_analytics.config import Settings
from kotoro_analytics.infrastructure.database.health import database_is_reachable
from kotoro_analytics.infrastructure.security.jwt import InvalidTokenError, decode_user_id

logger = logging.getLogger(__name__)


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


# auto_error=False so a missing/non-Bearer header becomes our own 401 body.
bearer_scheme = HTTPBearer(
    auto_error=False,
    scheme_name="KotoroBearer",
    description="Access token issued by the Kotoro API (`POST /auth/login`).",
)


def get_current_user_id(
    settings: SettingsDep,
    credentials: Annotated[HTTPAuthorizationCredentials | None, Security(bearer_scheme)],
) -> uuid.UUID:
    """Identity of the caller, taken only from a valid Kotoro access token."""
    if credentials is None:
        logger.info("request rejected: missing bearer token")
        raise UnauthorizedError

    try:
        return decode_user_id(
            credentials.credentials,
            secret=settings.jwt_signing_secret(),
            issuer=settings.jwt_issuer,
        )
    except InvalidTokenError as exc:
        # The reason is safe to log; the token itself never is.
        logger.info("request rejected: invalid token (%s)", exc)
        raise UnauthorizedError from exc


CurrentUserIdDep = Annotated[uuid.UUID, Depends(get_current_user_id)]
