from typing import Literal

from fastapi import APIRouter
from pydantic import BaseModel

router = APIRouter(tags=["infrastructure"])


class HealthResponse(BaseModel):
    status: Literal["ok"]


@router.get(
    "/healthz",
    response_model=HealthResponse,
    summary="Liveness check",
    description="Confirms the process is alive. Does not touch external dependencies.",
)
def healthz() -> HealthResponse:
    return HealthResponse(status="ok")
