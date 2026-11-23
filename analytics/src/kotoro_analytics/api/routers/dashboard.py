from fastapi import APIRouter

from kotoro_analytics.api.dependencies import (
    CurrentUserIdDep,
    GetDailySeriesDep,
    GetSummaryDep,
    PeriodDep,
)
from kotoro_analytics.api.errors import ERROR_RESPONSES
from kotoro_analytics.api.schemas import DailyResponse, SummaryResponse

router = APIRouter(prefix="/api/v1/analytics", tags=["dashboard"], responses=ERROR_RESPONSES)


@router.get(
    "/summary",
    response_model=SummaryResponse,
    summary="Dashboard summary for a period",
    description=(
        "Totals of the authenticated user's finished study cycles between `from` and `to` "
        "(both inclusive, calendar days in the metrics time zone), plus the current and "
        "longest streaks. Users without activity get zeros, never 404. Data is eventually "
        "consistent: a cycle finished a few seconds ago may not be counted yet."
    ),
)
async def get_summary(
    # Declared first so an unauthenticated request gets 401 before any validation detail.
    user_id: CurrentUserIdDep,
    period: PeriodDep,
    query: GetSummaryDep,
) -> SummaryResponse:
    return SummaryResponse.from_summary(await query.execute(user_id, period))


@router.get(
    "/daily",
    response_model=DailyResponse,
    summary="Daily series for charts",
    description=(
        "One entry per calendar day between `from` and `to` (inclusive, metrics time zone), "
        "ascending. Days without finished cycles are included with zeros, so the array can "
        "be plotted as is."
    ),
)
async def get_daily(
    user_id: CurrentUserIdDep,
    period: PeriodDep,
    query: GetDailySeriesDep,
) -> DailyResponse:
    return DailyResponse.from_series(await query.execute(user_id, period))
