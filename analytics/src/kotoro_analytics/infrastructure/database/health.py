import asyncio
import logging

from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession

logger = logging.getLogger(__name__)


async def database_is_reachable(session: AsyncSession, timeout_seconds: float) -> bool:
    """Run a trivial query within `timeout_seconds`. Never raises."""
    try:
        async with asyncio.timeout(timeout_seconds):
            await session.execute(text("SELECT 1"))
    except Exception as exc:  # noqa: BLE001 - any failure means "not ready"
        # Only the exception type is logged: messages can carry connection details.
        logger.warning("database readiness check failed: %s", type(exc).__name__)
        return False
    return True
