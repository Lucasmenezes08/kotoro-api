import base64
import json
import uuid
from datetime import UTC, datetime, timedelta

import pytest
from tests.support.tokens import make_token

from kotoro_analytics.config import DEVELOPMENT_JWT_SECRET
from kotoro_analytics.infrastructure.security.jwt import InvalidTokenError, decode_user_id

USER = uuid.UUID("11111111-1111-4111-8111-111111111111")


def decode(token: str) -> uuid.UUID:
    return decode_user_id(token, secret=DEVELOPMENT_JWT_SECRET, issuer="kotoro-api")


def _b64(data: dict) -> str:
    return base64.urlsafe_b64encode(json.dumps(data).encode()).rstrip(b"=").decode()


def test_valid_go_style_token_returns_user_id() -> None:
    assert decode(make_token(USER)) == USER


@pytest.mark.parametrize(
    ("token_kwargs", "reason"),
    [
        ({"secret": "another-secret-with-enough-length-0002"}, "wrong signature"),
        ({"expires_in": timedelta(minutes=-5)}, "expired"),
        ({"not_before": datetime.now(UTC) + timedelta(minutes=5)}, "not yet valid"),
        ({"issuer": "someone-else"}, "wrong issuer"),
        ({"issuer": None}, "missing issuer"),
        ({"drop": ("exp",)}, "missing exp"),
        ({"drop": ("sub",)}, "missing sub"),
        ({"sub": "not-a-uuid"}, "sub is not a UUID"),
        ({"sub": str(uuid.UUID(int=0)), "user_id": str(uuid.UUID(int=0))}, "nil UUID"),
        ({"user_id": str(uuid.uuid4())}, "user_id differs from sub"),
        ({"algorithm": "HS512"}, "algorithm other than HS256"),
    ],
)
@pytest.mark.filterwarnings("ignore::jwt.warnings.InsecureKeyLengthWarning")  # HS512 case
def test_untrusted_tokens_are_rejected(token_kwargs: dict, reason: str) -> None:
    with pytest.raises(InvalidTokenError):
        decode(make_token(USER, **token_kwargs))


def test_unsigned_alg_none_token_is_rejected() -> None:
    header = _b64({"alg": "none", "typ": "JWT"})
    payload = _b64(
        {"sub": str(USER), "user_id": str(USER), "iss": "kotoro-api", "exp": 9_999_999_999}
    )

    with pytest.raises(InvalidTokenError):
        decode(f"{header}.{payload}.")


def test_garbage_is_rejected() -> None:
    with pytest.raises(InvalidTokenError):
        decode("not.a.jwt")


def test_small_clock_skew_is_tolerated() -> None:
    token = make_token(USER, not_before=datetime.now(UTC) + timedelta(seconds=10))

    assert decode(token) == USER


def test_error_message_never_contains_the_token() -> None:
    token = make_token(USER, secret="another-secret-with-enough-length-0002")

    with pytest.raises(InvalidTokenError) as caught:
        decode(token)

    assert token not in str(caught.value)
