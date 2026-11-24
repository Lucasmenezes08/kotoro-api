import os
from typing import NoReturn

import pytest

# CI sets this so a missing PostgreSQL/RabbitMQ fails the build instead of skipping
# the integration tests silently.
REQUIRE_INTEGRATION = os.environ.get("ANALYTICS_REQUIRE_INTEGRATION") == "1"


def unavailable(service: str, error: BaseException) -> NoReturn:
    reason = f"{service} not reachable for integration tests ({type(error).__name__})"
    if REQUIRE_INTEGRATION:
        pytest.fail(f"{reason}; ANALYTICS_REQUIRE_INTEGRATION=1", pytrace=False)
    pytest.skip(reason)
