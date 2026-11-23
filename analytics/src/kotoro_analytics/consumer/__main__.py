"""Entry point: python -m kotoro_analytics.consumer"""

import asyncio
import contextlib
import signal

from kotoro_analytics.config import Settings
from kotoro_analytics.consumer.runner import run_consumer
from kotoro_analytics.observability.logging import configure_logging


async def _main() -> None:
    settings = Settings()
    configure_logging(settings)
    stop = asyncio.Event()
    loop = asyncio.get_running_loop()
    for sig in (signal.SIGINT, signal.SIGTERM):
        # Windows has no add_signal_handler; Ctrl+C still raises KeyboardInterrupt there.
        with contextlib.suppress(NotImplementedError):
            loop.add_signal_handler(sig, stop.set)
    await run_consumer(settings, stop)


if __name__ == "__main__":
    with contextlib.suppress(KeyboardInterrupt):
        asyncio.run(_main())
