from functools import lru_cache
from typing import Literal, Self
from zoneinfo import ZoneInfo, ZoneInfoNotFoundError

from pydantic import Field, SecretStr, field_validator, model_validator
from pydantic_settings import BaseSettings, SettingsConfigDict

LOCAL_DATABASE_URL = "postgresql+asyncpg://analytics:analytics@localhost:5433/kotoro_analytics"

# Used only when ANALYTICS_ENVIRONMENT is local/test and no secret is configured.
# docker-compose.yml passes the same value to the Go API as JWT_SECRET.
DEVELOPMENT_JWT_SECRET = "kotoro-local-development-jwt-secret-0001"
MIN_PRODUCTION_SECRET_LENGTH = 32


class Settings(BaseSettings):
    """Service configuration loaded from environment variables prefixed with ANALYTICS_."""

    model_config = SettingsConfigDict(env_prefix="ANALYTICS_", extra="ignore")

    app_name: str = "kotoro-analytics"
    environment: Literal["local", "test", "production"] = "local"
    log_level: Literal["debug", "info", "warning", "error"] = "info"
    # json for containers/log collectors; text is easier to read in a local terminal.
    log_format: Literal["json", "text"] = "json"

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

    # Must equal the Go API's JWT_SECRET (ADR 0002). SecretStr keeps it out of repr/logs.
    jwt_secret: SecretStr | None = None
    jwt_issuer: str = "kotoro-api"

    # Analytics Consumer (python -m kotoro_analytics.consumer). Topology matches the Go
    # relay's SetupTopology (internal/messaging/rabbitmq/publisher.go).
    rabbitmq_url: str = Field(default="amqp://guest:guest@localhost:5672/", repr=False)
    rabbitmq_exchange: str = "kotoro.domain-events"
    rabbitmq_queue: str = "kotoro.analytics.events"
    rabbitmq_prefetch: int = Field(default=10, ge=1)
    consumer_retry_delay_seconds: float = Field(default=5.0, gt=0)

    @model_validator(mode="after")
    def _require_jwt_secret_outside_development(self) -> Self:
        if self.jwt_secret is None or not self.jwt_secret.get_secret_value():
            if self.environment == "production":
                raise ValueError("ANALYTICS_JWT_SECRET is required in production")
            self.jwt_secret = SecretStr(DEVELOPMENT_JWT_SECRET)
        elif (
            self.environment == "production"
            and len(self.jwt_secret.get_secret_value()) < MIN_PRODUCTION_SECRET_LENGTH
        ):
            raise ValueError(
                f"ANALYTICS_JWT_SECRET must have at least {MIN_PRODUCTION_SECRET_LENGTH} characters"
            )
        return self

    def jwt_signing_secret(self) -> str:
        assert self.jwt_secret is not None  # guaranteed by the validator above
        return self.jwt_secret.get_secret_value()

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
