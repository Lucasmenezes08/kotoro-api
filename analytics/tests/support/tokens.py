"""Builds tokens shaped like the Go API's (internal/auth/token.go GenerateToken)."""

import uuid
from datetime import UTC, datetime, timedelta
from typing import Any

import jwt

from kotoro_analytics.config import DEVELOPMENT_JWT_SECRET


def make_token(
    subject: uuid.UUID | str,
    /,
    *,
    secret: str = DEVELOPMENT_JWT_SECRET,
    algorithm: str = "HS256",
    issuer: str | None = "kotoro-api",
    expires_in: timedelta = timedelta(hours=24),
    not_before: datetime | None = None,
    drop: tuple[str, ...] = (),
    **overrides: Any,
) -> str:
    now = datetime.now(UTC)
    claims: dict[str, Any] = {
        "user_id": str(subject),
        "email": "student@example.com",
        "sub": str(subject),
        "iat": now,
        "exp": now + expires_in,
        "nbf": not_before or now,
        "iss": issuer,
    }
    claims.update(overrides)
    for name in drop:
        claims.pop(name, None)
    if claims.get("iss") is None:
        claims.pop("iss", None)
    return jwt.encode(claims, secret, algorithm=algorithm)
