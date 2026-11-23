import pytest
from pydantic import ValidationError

from kotoro_analytics.config import Settings


def test_default_metrics_timezone_is_sao_paulo() -> None:
    assert Settings().metrics_timezone == "America/Sao_Paulo"


def test_unknown_metrics_timezone_is_rejected() -> None:
    with pytest.raises(ValidationError, match="unknown IANA time zone"):
        Settings(metrics_timezone="Mars/Olympus_Mons")


def test_database_url_is_hidden_from_repr() -> None:
    settings = Settings(database_url="postgresql+asyncpg://user:pa55w0rd-db@db:5432/x")

    assert "pa55w0rd-db" not in repr(settings)


def test_local_environment_falls_back_to_development_jwt_secret() -> None:
    from kotoro_analytics.config import DEVELOPMENT_JWT_SECRET

    assert Settings(environment="local").jwt_signing_secret() == DEVELOPMENT_JWT_SECRET


def test_production_requires_jwt_secret() -> None:
    with pytest.raises(ValidationError, match="ANALYTICS_JWT_SECRET is required"):
        Settings(environment="production")


def test_production_rejects_short_jwt_secret() -> None:
    with pytest.raises(ValidationError, match="at least 32 characters"):
        Settings(environment="production", jwt_secret="short")


def test_jwt_secret_is_hidden_from_repr() -> None:
    settings = Settings(jwt_secret="super-secret-value-that-is-long-enough-123")

    assert "super-secret-value" not in repr(settings)
