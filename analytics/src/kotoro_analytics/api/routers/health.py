from typing import Literal

from fastapi import APIRouter, Response, status
from pydantic import BaseModel

from kotoro_analytics.api.dependencies import DatabaseReadyDep

router = APIRouter(tags=["infrastructure"])

CheckStatus = Literal["ok", "unavailable"]


class HealthResponse(BaseModel):
    status: Literal["ok"]


class ReadinessChecks(BaseModel):
    database: CheckStatus


class ReadinessResponse(BaseModel):
    status: Literal["ready", "unavailable"]
    checks: ReadinessChecks


@router.get(
    "/healthz",
    response_model=HealthResponse,
    summary="Liveness check",
    description="Confirms the process is alive. Does not touch external dependencies.",
)
def healthz() -> HealthResponse:
    return HealthResponse(status="ok")


@router.get(
    "/readyz",
    response_model=ReadinessResponse,
    summary="Readiness check",
    description=(
        "Confirms the service can serve requests: the analytics database answers a "
        "trivial query within the readiness timeout. Returns 503 otherwise."
    ),
    responses={
        status.HTTP_503_SERVICE_UNAVAILABLE: {
            "model": ReadinessResponse,
            "description": "A dependency is unavailable.",
            "content": {
                "application/json": {
                    "example": {"status": "unavailable", "checks": {"database": "unavailable"}}
                }
            },
        }
    },
)
async def readyz(response: Response, database_ready: DatabaseReadyDep) -> ReadinessResponse:
    if database_ready:
        return ReadinessResponse(status="ready", checks=ReadinessChecks(database="ok"))

    response.status_code = status.HTTP_503_SERVICE_UNAVAILABLE
    return ReadinessResponse(status="unavailable", checks=ReadinessChecks(database="unavailable"))
