from fastapi import FastAPI

from kotoro_analytics.api.routers import health
from kotoro_analytics.config import Settings, get_settings


def create_app(settings: Settings | None = None) -> FastAPI:
    settings = settings or get_settings()

    app = FastAPI(
        title="Kotoro Analytics API",
        version="0.1.0",
        description="Read-only queries over the Kotoro analytics projections.",
    )
    app.state.settings = settings
    app.include_router(health.router)
    return app


app = create_app()
