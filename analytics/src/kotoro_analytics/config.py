from functools import lru_cache
from typing import Literal
from zoneinfo import ZoneInfo, ZoneInfoNotFoundError

from pydantic import Field, field_validator
from pydantic_settings import BaseSettings, SettingsConfigDict

LOCAL_DATABASE_URL = "postgresql+asyncpg://analytics:analytics@localhost:5433/kotoro_analytics"


class Settings(BaseSettings):
    """Service configuration loaded from environment variables prefixed with ANALYTICS_."""

    model_config = SettingsConfigDict(env_prefix="ANALYTICS_", extra="ignore")

    app_name: str = "kotoro-analytics"
    environment: Literal["local", "test", "production"] = "local"
    log_level: Literal["debug", "info", "warning", "error"] = "info"

    # repr=False keeps the password out of logs and error messages.
    database_url: str = Field(default=LOCAL_DATABASE_URL, repr=False)
    metrics_timezone: str = "America/Sao_Paulo"

    # Connection pool. The API is read-only, so a small pool is enough.
    db_pool_size: int = Field(default=5, ge=1)
    db_max_overflow: int = Field(default=5, ge=0)
    db_pool_timeout_seconds: float = Field(default=5.0, gt=0)
    db_connect_timeout_seconds: float = Field(default=5.0, gt=0)
    db_statement_timeout_seconds: float = Field(default=10.0, gt=0)
    db_readiness_timeout_seconds: float = Field(default=2.0, gt=0)

    @field_validator("metrics_timezone")
    @classmethod
    def _validate_timezone(cls, value: str) -> str:
        try:
            ZoneInfo(value)
        except (ZoneInfoNotFoundError, ValueError) as exc:
            raise ValueError(f"unknown IANA time zone: {value!r}") from exc
        return value


@lru_cache
def get_settings() -> Settings:
    return Settings()
