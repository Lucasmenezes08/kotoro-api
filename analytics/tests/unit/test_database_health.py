import asyncio

import pytest

from kotoro_analytics.infrastructure.database.health import database_is_reachable


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


class _FakeSession:
    def __init__(self, *, error: Exception | None = None, delay: float = 0.0) -> None:
        self._error = error
        self._delay = delay

    async def execute(self, _statement: object) -> None:
        await asyncio.sleep(self._delay)
        if self._error is not None:
            raise self._error


@pytest.mark.anyio
async def test_reachable_when_query_succeeds() -> None:
    assert await database_is_reachable(_FakeSession(), timeout_seconds=1) is True


@pytest.mark.anyio
async def test_unreachable_when_query_fails() -> None:
    session = _FakeSession(error=ConnectionRefusedError("refused"))

    assert await database_is_reachable(session, timeout_seconds=1) is False


@pytest.mark.anyio
async def test_unreachable_when_query_exceeds_timeout() -> None:
    session = _FakeSession(delay=0.5)

    assert await database_is_reachable(session, timeout_seconds=0.05) is False


@pytest.mark.anyio
async def test_failure_log_does_not_leak_error_message(caplog: pytest.LogCaptureFixture) -> None:
    session = _FakeSession(error=OSError("password=secret host=db"))

    await database_is_reachable(session, timeout_seconds=1)

    assert "secret" not in caplog.text
    assert "OSError" in caplog.text
