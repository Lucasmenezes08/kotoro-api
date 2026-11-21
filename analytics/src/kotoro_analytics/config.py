from functools import lru_cache
from typing import Literal

from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """Service configuration loaded from environment variables prefixed with ANALYTICS_."""

    model_config = SettingsConfigDict(env_prefix="ANALYTICS_", extra="ignore")

    app_name: str = "kotoro-analytics"
    environment: Literal["local", "test", "production"] = "local"
    log_level: Literal["debug", "info", "warning", "error"] = "info"


@lru_cache
def get_settings() -> Settings:
    return Settings()
