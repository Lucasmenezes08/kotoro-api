import pytest
from pydantic import ValidationError

from kotoro_analytics.config import Settings


def test_default_metrics_timezone_is_sao_paulo() -> None:
    assert Settings().metrics_timezone == "America/Sao_Paulo"


def test_unknown_metrics_timezone_is_rejected() -> None:
    with pytest.raises(ValidationError, match="unknown IANA time zone"):
        Settings(metrics_timezone="Mars/Olympus_Mons")


def test_database_url_is_hidden_from_repr() -> None:
    settings = Settings(database_url="postgresql+asyncpg://user:secret@db:5432/x")

    assert "secret" not in repr(settings)
