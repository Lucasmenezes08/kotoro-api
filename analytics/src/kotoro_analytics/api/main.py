from collections.abc import AsyncIterator
from contextlib import asynccontextmanager

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from kotoro_analytics.api.errors import register_error_handlers
from kotoro_analytics.api.middleware import RequestIdMiddleware
from kotoro_analytics.api.routers import dashboard, health
from kotoro_analytics.config import Settings, get_settings
from kotoro_analytics.infrastructure.database.session import (
    create_engine,
    create_session_factory,
)
from kotoro_analytics.observability.logging import configure_logging


def create_app(settings: Settings | None = None) -> FastAPI:
    settings = settings or get_settings()
    if settings.environment != "test":
        # Tests configure logging themselves (pytest captures the root logger).
        configure_logging(settings)

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
    app.add_middleware(RequestIdMiddleware)
    app.add_middleware(
        CORSMiddleware,
        allow_origins=settings.cors_allowed_origins,
        allow_credentials=True,
        allow_methods=["GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"],
        allow_headers=["Accept", "Content-Type", "Authorization", "X-Request-ID"],
        expose_headers=["X-Request-ID"],
    )
    app.include_router(health.router)
    app.include_router(dashboard.router)
    return app


app = create_app()
