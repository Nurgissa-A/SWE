from concurrent.futures import ThreadPoolExecutor
from datetime import UTC, datetime, timedelta
from uuid import UUID

import jwt
import pytest
from conftest import logged_in, registered
from cryptography.hazmat.primitives.asymmetric import rsa
from sqlalchemy import select

from app.api.dependencies import require_roles
from app.core.security import TokenService, verify_password
from app.models.identity import ActionToken, Role, User


def test_registration_normalizes_persists_and_hashes(ctx):
    client, app, mailer = ctx
    result = client.post(
        "/api/auth/register",
        json={"email": " Person@Example.COM ", "password": "safe-password-123"},
    )
    assert result.status_code == 201
    body = result.json()
    assert body["email"] == "person@example.com" and body["role"] == "attendee"
    assert body["emailVerified"] is False and UUID(body["id"])
    assert "password" not in result.text and "token" not in result.text
    with app.state.session_factory() as session:
        user = session.scalar(select(User))
        assert user.password_hash.startswith("$argon2id$")
        assert verify_password(user.password_hash, "safe-password-123")
        action = session.scalar(select(ActionToken))
        assert mailer.messages[0][2] != action.token_hash
        assert len(action.token_hash) == 64


def test_duplicate_case_insensitive(ctx):
    registered(ctx)
    result = ctx[0].post(
        "/api/auth/register", json={"email": "PERSON@example.com", "password": "another-password"}
    )
    assert result.status_code == 409
    assert len(ctx[2].messages) == 1


@pytest.mark.parametrize(
    "payload",
    [
        {},
        {"email": "bad", "password": "long-enough"},
        {"email": "a@example.com", "password": "short"},
        {"email": "a@example.com", "password": "x" * 1025},
        {"email": "a@example.com", "password": "long-enough", "role": "platform_admin"},
        {"email": "a@example.com", "password": "long-enough", "id": "forged"},
    ],
)
def test_invalid_registration_and_privilege_escalation(ctx, payload):
    response = ctx[0].post("/api/auth/register", json=payload)
    assert response.status_code == 422
    assert "password" not in response.text and "input" not in response.text


def test_verification_single_use_and_login(ctx):
    registered(ctx, verify=False)
    client, _, mailer = ctx
    assert (
        client.post(
            "/api/auth/login", json={"email": "person@example.com", "password": "safe-password-123"}
        ).status_code
        == 403
    )
    token = mailer.messages[0][2]
    assert client.post("/api/auth/verify-email", json={"token": token}).status_code == 200
    assert client.post("/api/auth/verify-email", json={"token": token}).status_code == 400
    login = client.post(
        "/api/auth/login", json={"email": "person@example.com", "password": "safe-password-123"}
    )
    assert login.status_code == 200 and login.json()["user"]["emailVerified"]
    assert login.headers["Cache-Control"] == "no-store"


@pytest.mark.parametrize("purpose", ["verify_email", "reset_password"])
def test_expired_action_token(ctx, purpose):
    registered(ctx, verify=False)
    if purpose == "reset_password":
        ctx[0].post("/api/auth/request-password-reset", json={"email": "person@example.com"})
    with ctx[1].state.session_factory() as session:
        row = session.scalar(select(ActionToken).where(ActionToken.purpose == purpose))
        row.expires_at = datetime.now(UTC) - timedelta(seconds=1)
        session.commit()
    payload = {"token": ctx[2].messages[-1][2]}
    endpoint = "/api/auth/verify-email"
    if purpose == "reset_password":
        endpoint = "/api/auth/reset-password"
        payload["newPassword"] = "new-password-123"
    assert ctx[0].post(endpoint, json=payload).status_code == 400


def test_invalid_and_wrong_purpose_tokens(ctx):
    registered(ctx)
    assert ctx[0].post("/api/auth/verify-email", json={"token": "invalid"}).status_code == 400
    ctx[0].post("/api/auth/request-password-reset", json={"email": "person@example.com"})
    assert (
        ctx[0].post("/api/auth/verify-email", json={"token": ctx[2].messages[-1][2]}).status_code
        == 400
    )


def test_concurrent_verification_consumes_once(ctx):
    registered(ctx, verify=False)
    token = ctx[2].messages[-1][2]
    with ThreadPoolExecutor(max_workers=2) as pool:
        statuses = list(
            pool.map(
                lambda _: ctx[0].post("/api/auth/verify-email", json={"token": token}).status_code,
                range(2),
            )
        )
    assert sorted(statuses) == [200, 400]


@pytest.mark.parametrize(
    "email,password", [("person@example.com", "wrong"), ("unknown@example.com", "wrong")]
)
def test_invalid_login(ctx, email, password):
    registered(ctx)
    response = ctx[0].post("/api/auth/login", json={"email": email, "password": password})
    assert response.status_code == 401 and response.headers["WWW-Authenticate"] == "Bearer"


def test_me_and_update(ctx):
    headers = logged_in(ctx)
    assert ctx[0].get("/api/users/me", headers=headers).json()["email"] == "person@example.com"
    result = ctx[0].patch("/api/users/me", headers=headers, json={"displayName": "My Name"})
    assert result.status_code == 200 and result.json()["displayName"] == "My Name"
    assert (
        ctx[0].patch("/api/users/me", headers=headers, json={"role": "platform_admin"}).status_code
        == 422
    )


@pytest.mark.parametrize(
    "header", [{}, {"Authorization": "Bearer bad"}, {"Authorization": "Basic abc"}]
)
def test_missing_or_invalid_bearer(ctx, header):
    response = ctx[0].get("/api/users/me", headers=header)
    assert response.status_code == 401


@pytest.mark.parametrize(
    "mutation",
    [
        "expired",
        "issuer",
        "audience",
        "signature",
        "algorithm",
        "kid",
        "missing",
        "subject",
        "version",
    ],
)
def test_jwt_rejections(ctx, mutation):
    headers = logged_in(ctx)
    app = ctx[1]
    good = headers["Authorization"].split()[1]
    claims = app.state.tokens.decode(good)
    key = app.state.tokens.private_key
    algorithm, kid = "RS256", app.state.settings.jwt_key_id
    if mutation == "expired":
        claims["exp"] = datetime.now(UTC) - timedelta(seconds=1)
    if mutation == "issuer":
        claims["iss"] = "wrong"
    if mutation == "audience":
        claims["aud"] = "wrong"
    if mutation == "signature":
        key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    if mutation == "algorithm":
        algorithm, key = "HS256", "different-test-signing-secret-at-least-32-bytes"
    if mutation == "kid":
        kid = "unknown"
    if mutation == "missing":
        del claims["exp"]
    if mutation == "subject":
        claims["sub"] = "not-a-uuid"
    if mutation == "version":
        claims["ver"] = -1
    token = jwt.encode(claims, key, algorithm=algorithm, headers={"kid": kid})
    assert (
        ctx[0].get("/api/users/me", headers={"Authorization": "Bearer " + token}).status_code == 401
    )


@pytest.mark.parametrize("state", ["disabled", "deleted", "revoked"])
def test_inactive_or_removed_user(ctx, state):
    headers = logged_in(ctx)
    with ctx[1].state.session_factory() as session:
        user = session.scalar(select(User))
        if state == "disabled":
            user.is_active = False
        if state == "deleted":
            session.delete(user)
        if state == "revoked":
            user.token_version += 1
        session.commit()
    assert ctx[0].get("/api/users/me", headers=headers).status_code == 401


@pytest.mark.parametrize("role", list(Role))
def test_all_roles_and_explicit_permission_dependencies(ctx, role):
    logged_in(ctx)
    from fastapi import Depends

    with ctx[1].state.session_factory() as session:
        user = session.scalar(select(User))
        user.role = role.value
        session.commit()
    result = ctx[0].post(
        "/api/auth/login", json={"email": "person@example.com", "password": "safe-password-123"}
    )
    headers = {"Authorization": "Bearer " + result.json()["token"]}

    @ctx[1].get("/test/organizer-only")
    def organizer_only(user=Depends(require_roles(Role.ORGANIZER))):
        return {"role": user.role}

    assert ctx[0].get("/api/users/me", headers=headers).json()["role"] == role.value
    assert ctx[0].get("/test/organizer-only", headers=headers).status_code == (
        200 if role == Role.ORGANIZER else 403
    )


def test_password_reset_and_revocation(ctx):
    headers = logged_in(ctx)
    assert (
        ctx[0]
        .post("/api/auth/request-password-reset", json={"email": "missing@example.com"})
        .status_code
        == 200
    )
    ctx[0].post("/api/auth/request-password-reset", json={"email": "person@example.com"})
    token = ctx[2].messages[-1][2]
    payload = {"token": token, "newPassword": "changed-password-123"}
    assert ctx[0].post("/api/auth/reset-password", json=payload).status_code == 200
    assert ctx[0].post("/api/auth/reset-password", json=payload).status_code == 400
    assert ctx[0].get("/api/users/me", headers=headers).status_code == 401
    assert (
        ctx[0]
        .post(
            "/api/auth/login", json={"email": "person@example.com", "password": "safe-password-123"}
        )
        .status_code
        == 401
    )
    assert (
        ctx[0]
        .post(
            "/api/auth/login",
            json={"email": "person@example.com", "password": "changed-password-123"},
        )
        .status_code
        == 200
    )


def test_mail_failure_rolls_back_registration(ctx):
    def fail(*args):
        raise OSError("provider unavailable")

    ctx[1].state.mailer.send = fail
    response = ctx[0].post(
        "/api/auth/register", json={"email": "mail@example.com", "password": "safe-password-123"}
    )
    assert response.status_code == 503
    with ctx[1].state.session_factory() as session:
        assert session.scalar(select(User)) is None


def test_jwks_public_only(ctx):
    keys = ctx[0].get("/.well-known/jwks.json").json()["keys"]
    assert keys[0]["alg"] == "RS256" and "d" not in keys[0]
    assert "PRIVATE" not in str(keys)


def test_mismatched_keys_rejected(settings, tmp_path):
    public = tmp_path / "wrong.pem"
    from cryptography.hazmat.primitives import serialization

    key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    public.write_bytes(
        key.public_key().public_bytes(
            serialization.Encoding.PEM, serialization.PublicFormat.SubjectPublicKeyInfo
        )
    )
    with pytest.raises(ValueError, match="does not match"):
        TokenService(settings.model_copy(update={"jwt_public_key_file": public}))
