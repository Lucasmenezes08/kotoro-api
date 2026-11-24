FROM python:3.12-slim AS builder

COPY --from=ghcr.io/astral-sh/uv:0.12.23 /uv /usr/local/bin/uv

ENV UV_COMPILE_BYTECODE=1 \
    UV_LINK_MODE=copy

WORKDIR /app

COPY pyproject.toml uv.lock README.md ./
RUN uv sync --frozen --no-dev --no-install-project

COPY src ./src
RUN uv sync --frozen --no-dev

COPY alembic.ini ./
COPY migrations ./migrations


FROM python:3.12-slim

RUN useradd --create-home --uid 1000 analytics

WORKDIR /app
COPY --from=builder --chown=analytics:analytics /app /app

ENV PATH="/app/.venv/bin:$PATH" \
    PYTHONUNBUFFERED=1

USER analytics
EXPOSE 8001

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD python -c "import urllib.request; urllib.request.urlopen('http://localhost:8001/healthz', timeout=2)"

# Access lines come from RequestIdMiddleware (with request id, without query strings).
CMD ["uvicorn", "kotoro_analytics.api.main:app", "--host", "0.0.0.0", "--port", "8001", "--no-access-log"]
