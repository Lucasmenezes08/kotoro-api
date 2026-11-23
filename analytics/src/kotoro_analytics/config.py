from functools import lru_cache
from typing import Literal

from pydantic import Field
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


@lru_cache
def get_settings() -> Settings:
    return Settings()
