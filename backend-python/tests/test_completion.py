"""Regression coverage for Aliar's remaining concurrent identity boundaries."""

import time
from concurrent.futures import ThreadPoolExecutor
from uuid import UUID, uuid4

import jwt
import pytest
from conftest import logged_in
from fastapi import HTTPException
from sqlalchemy import create_engine, event, select, text
from sqlalchemy.orm import Session

from app.models.identity import ActionToken, User
from app.services.identity import IdentityService


def wait_for_blocked(engine, application, count):
    deadline = time.monotonic() + 5
    while time.monotonic() < deadline:
        with engine.connect() as conn:
            waiting = conn.scalar(
                text(
                    "SELECT count(*) FROM pg_stat_activity "
                    "WHERE application_name = :name AND wait_event_type = 'Lock'"
                ),
                {"name": application},
            )
        if waiting == count:
            return
        time.sleep(0.01)
    pytest.fail("Concurrent requests did not reach the database lock")


def test_two_reset_links_serialize_without_deadlock(ctx, database_url):
    logged_in(ctx)
    for _ in range(2):
        assert (
            ctx[0]
            .post("/api/auth/request-password-reset", json={"email": "person@example.com"})
            .status_code
            == 200
        )
    links = [message[2] for message in ctx[2].messages if message[1] == "reset_password"]
    label = "reset-race-" + uuid4().hex
    engine = create_engine(database_url, connect_args={"application_name": label})

    def reset(raw):
        with Session(engine) as session:
            session.execute(text("SET LOCAL lock_timeout = '5s'"))
            try:
                IdentityService(session, ctx[1].state.settings, ctx[2]).reset(
                    raw, "changed-password-123"
                )
                return 200
            except HTTPException as exc:
                return exc.status_code
            except Exception as exc:
                return type(exc).__name__

    try:
        with ctx[1].state.session_factory() as blocker:
            user = blocker.scalar(select(User).with_for_update())
            uid = user.id
            with ThreadPoolExecutor(max_workers=2) as pool:
                futures = [pool.submit(reset, raw) for raw in links]
                try:
                    wait_for_blocked(ctx[1].state.engine, label, 2)
                finally:
                    blocker.rollback()
                statuses = [future.result(timeout=8) for future in futures]
        assert sorted(statuses, key=str) == [200, 400], statuses
        with ctx[1].state.session_factory() as session:
            assert session.get(User, uid).token_version == 1
            assert all(
                row.used_at is not None
                for row in session.scalars(
                    select(ActionToken).where(ActionToken.purpose == "reset_password")
                )
            )
    finally:
        engine.dispose()


@pytest.mark.parametrize("action", ["disable", "promote_admin", "revoke"])
def test_waiting_onboarding_rechecks_identity(ctx, action):
    headers = logged_in(ctx)
    engine = ctx[1].state.engine
    # Label the app's connections so the observer can wait for an actual DB lock.
    label = "onboard-race-" + uuid4().hex

    @event.listens_for(engine, "checkout")
    def label_connection(connection, record, proxy):
        with connection.cursor() as cursor:
            cursor.execute("SELECT set_config('application_name', %s, false)", (label,))
        connection.commit()

    with ctx[1].state.session_factory() as blocker:
        user = blocker.scalar(select(User).with_for_update())
        with ThreadPoolExecutor(max_workers=1) as pool:
            future = pool.submit(
                ctx[0].post,
                "/api/organizer-profile",
                headers=headers,
                json={"contactEmail": "person@example.com"},
            )
            try:
                wait_for_blocked(engine, label, 1)
                if action == "disable":
                    user.is_active = False
                elif action == "promote_admin":
                    user.role = "platform_admin"
                else:
                    user.token_version += 1
                blocker.commit()
            finally:
                blocker.rollback()
            response = future.result(timeout=8)
    assert response.status_code == (403 if action == "promote_admin" else 401)
    with ctx[1].state.session_factory() as session:
        saved = session.get(User, user.id)
        assert saved.role == ("platform_admin" if action == "promote_admin" else "attendee")


@pytest.mark.parametrize(
    "mutation", ["bad_jti", "compact_sub", "compact_jti", "boolean_iat", "boolean_nbf"]
)
def test_python_jwt_claim_contract_matches_go(ctx, mutation):
    headers = logged_in(ctx)
    tokens = ctx[1].state.tokens
    claims = tokens.decode(headers["Authorization"].split()[1])
    if mutation == "bad_jti":
        claims["jti"] = "not-a-uuid"
    elif mutation == "compact_sub":
        claims["sub"] = UUID(claims["sub"]).hex
    elif mutation == "compact_jti":
        claims["jti"] = UUID(claims["jti"]).hex
    elif mutation == "boolean_iat":
        claims["iat"] = True
    else:
        claims["nbf"] = True
    token = jwt.encode(
        claims,
        tokens.private_key,
        algorithm="RS256",
        headers={"kid": ctx[1].state.settings.jwt_key_id},
    )
    response = ctx[0].get("/api/users/me", headers={"Authorization": "Bearer " + token})
    assert response.status_code == 401


def test_database_lock_timeout_is_sanitized_503(ctx):
    headers = logged_in(ctx)
    engine = ctx[1].state.engine

    @event.listens_for(engine, "checkout")
    def short_lock_timeout(connection, record, proxy):
        with connection.cursor() as cursor:
            cursor.execute("SET lock_timeout = '100ms'")
        connection.commit()

    with ctx[1].state.session_factory() as blocker:
        user = blocker.scalar(select(User).with_for_update())
        response = ctx[0].patch(
            "/api/users/me", headers=headers, json={"displayName": "must not persist"}
        )
        assert response.status_code == 503
        assert response.json()["error"]["code"] == "503"
        assert "SELECT" not in response.text and "postgresql" not in response.text
        assert user.display_name is None


@pytest.mark.parametrize("field", ["iat", "nbf", "exp"])
@pytest.mark.parametrize("value", [float("inf"), float("nan")])
def test_nonfinite_jwt_dates_are_unauthorized(ctx, field, value):
    headers = logged_in(ctx)
    tokens = ctx[1].state.tokens
    claims = tokens.decode(headers["Authorization"].split()[1])
    claims[field] = value
    token = jwt.encode(
        claims,
        tokens.private_key,
        algorithm="RS256",
        headers={"kid": ctx[1].state.settings.jwt_key_id},
    )
    assert (
        ctx[0].get("/api/users/me", headers={"Authorization": "Bearer " + token}).status_code == 401
    )


def test_fractional_numeric_dates_remain_supported(ctx):
    headers = logged_in(ctx)
    tokens = ctx[1].state.tokens
    claims = tokens.decode(headers["Authorization"].split()[1])
    claims["iat"] -= 0.5
    claims["nbf"] -= 0.5
    claims["exp"] += 0.5
    token = jwt.encode(
        claims,
        tokens.private_key,
        algorithm="RS256",
        headers={"kid": ctx[1].state.settings.jwt_key_id},
    )
    assert (
        ctx[0].get("/api/users/me", headers={"Authorization": "Bearer " + token}).status_code == 200
    )
