from collections.abc import AsyncIterator
from contextlib import asynccontextmanager

from fastapi import FastAPI

from kotoro_analytics.api.errors import register_error_handlers
from kotoro_analytics.api.routers import health
from kotoro_analytics.config import Settings, get_settings
from kotoro_analytics.infrastructure.database.session import (
    create_engine,
    create_session_factory,
)


def create_app(settings: Settings | None = None) -> FastAPI:
    settings = settings or get_settings()

    @asynccontextmanager
    async def lifespan(app: FastAPI) -> AsyncIterator[None]:
        # The engine connects lazily, so the app still starts when the database is down.
        engine = create_engine(settings)
        app.state.engine = engine
        app.state.session_factory = create_session_factory(engine)
        try:
            yield
        finally:
            await engine.dispose()

    app = FastAPI(
        title="Kotoro Analytics API",
        version="0.1.0",
        description="Read-only queries over the Kotoro analytics projections.",
        lifespan=lifespan,
    )
    app.state.settings = settings
    register_error_handlers(app)
    app.include_router(health.router)
    return app


app = create_app()
