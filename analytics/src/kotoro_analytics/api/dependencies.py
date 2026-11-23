import logging
import uuid
from collections.abc import AsyncIterator
from datetime import date
from typing import Annotated
from zoneinfo import ZoneInfo

from fastapi import Depends, Query, Request, Security
from fastapi.security import HTTPAuthorizationCredentials, HTTPBearer
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker

from kotoro_analytics.api.errors import UnauthorizedError
from kotoro_analytics.application.queries.daily import DailyReadRepository, GetDailySeries
from kotoro_analytics.application.queries.summary import (
    Clock,
    GetSummary,
    SummaryReadRepository,
    utc_now,
)
from kotoro_analytics.config import Settings
from kotoro_analytics.domain.period import Period
from kotoro_analytics.infrastructure.database.health import database_is_reachable
from kotoro_analytics.infrastructure.database.repositories import (
    PostgresDailyRepository,
    PostgresSummaryRepository,
)
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


def get_period(
    from_: Annotated[
        date,
        Query(
            alias="from", description="First day, inclusive (YYYY-MM-DD).", examples=["2026-11-01"]
        ),
    ],
    to: Annotated[
        date,
        Query(description="Last day, inclusive (YYYY-MM-DD).", examples=["2026-11-30"]),
    ],
) -> Period:
    """Inclusive period in the metrics time zone. Invalid ranges become 422 (ADR 0002)."""
    return Period(start=from_, end=to)


PeriodDep = Annotated[Period, Depends(get_period)]


def get_clock() -> Clock:
    """Current time source. Tests override it to pin "today"."""
    return utc_now


def get_summary_repository(session: SessionDep) -> SummaryReadRepository:
    return PostgresSummaryRepository(session)


def get_summary_query(
    repository: Annotated[SummaryReadRepository, Depends(get_summary_repository)],
    settings: SettingsDep,
    clock: Annotated[Clock, Depends(get_clock)],
) -> GetSummary:
    return GetSummary(repository, timezone=ZoneInfo(settings.metrics_timezone), clock=clock)


GetSummaryDep = Annotated[GetSummary, Depends(get_summary_query)]


def get_daily_repository(session: SessionDep) -> DailyReadRepository:
    return PostgresDailyRepository(session)


def get_daily_query(
    repository: Annotated[DailyReadRepository, Depends(get_daily_repository)],
    settings: SettingsDep,
) -> GetDailySeries:
    return GetDailySeries(repository, timezone=ZoneInfo(settings.metrics_timezone))


GetDailySeriesDep = Annotated[GetDailySeries, Depends(get_daily_query)]
