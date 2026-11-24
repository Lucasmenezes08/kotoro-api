"""Validation of the access tokens issued by the Kotoro Go API.

Contract (internal/auth/token.go in the Go API): HS256, `sub` = user id, a `user_id`
claim with the same UUID, `iss` = "kotoro-api", `exp`/`nbf`/`iat` set.
See docs/adr/0002-summary-contract-and-identity.md (decision 1).
"""

import uuid

import jwt

# Pinned: tokens signed with any other algorithm (including "none") are rejected.
ALGORITHM = "HS256"
# Tolerates small clock differences between the Go API and this service.
LEEWAY_SECONDS = 30


class InvalidTokenError(Exception):
    """The token cannot be trusted. The message is safe to log; it never contains the token."""


def decode_user_id(token: str, *, secret: str, issuer: str) -> uuid.UUID:
    try:
        claims = jwt.decode(
            token,
            secret,
            algorithms=[ALGORITHM],
            issuer=issuer,
            leeway=LEEWAY_SECONDS,
            options={"require": ["sub", "exp", "iss"]},
        )
    except jwt.PyJWTError as exc:
        raise InvalidTokenError(type(exc).__name__) from exc

    try:
        user_id = uuid.UUID(claims["sub"])
    except (TypeError, ValueError, AttributeError) as exc:
        raise InvalidTokenError("sub is not a UUID") from exc

    if user_id.int == 0:
        raise InvalidTokenError("sub is the nil UUID")

    # The Go API also writes `user_id`; when present it must agree with `sub`.
    if "user_id" in claims and str(claims["user_id"]).lower() != str(user_id):
        raise InvalidTokenError("user_id does not match sub")

    return user_id
