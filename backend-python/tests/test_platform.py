"""Real PostgreSQL coverage; injected Go boundary is explicitly a contract fixture."""

from concurrent.futures import ThreadPoolExecutor
from datetime import UTC, datetime, timedelta
from uuid import UUID, uuid4

import httpx
import pytest
from conftest import logged_in
from sqlalchemy import select, text

from app.models.identity import User
from app.models.platform import (
    Notification,
    SupportCase,
)
from app.services.business import BusinessClient


def role(ctx, email, role_name):
    headers = logged_in(ctx, email)
    uid = UUID(ctx[0].get("/api/users/me", headers=headers).json()["id"])
    with ctx[1].state.session_factory() as session:
        session.get(User, uid).role = role_name
        session.commit()
    return headers, uid


def key(headers):
    return {**headers, "Idempotency-Key": str(uuid4())}


def internal(ctx):
    return {"Authorization": "Bearer " + ctx[1].state.service_token}


def context_fixture(ctx, owner, event=None, staff=None):
    event = event or uuid4()

    def handle(request):
        if request.url.path == f"/api/events/{event}":
            return httpx.Response(200, json={"id": str(event), "organizerId": str(owner)})
        if request.url.path.endswith("/staff/me"):
            return (
                httpx.Response(
                    200, json={"userId": str(staff), "eventId": str(event), "canSupport": True}
                )
                if staff
                else httpx.Response(403)
            )
        return httpx.Response(404)

    ctx[1].state.business.client.close()
    ctx[1].state.business.client = httpx.Client(
        base_url="http://go-fixture", transport=httpx.MockTransport(handle)
    )
    return event


def test_logout_revokes_one_session_and_keeps_others(ctx):
    first = logged_in(ctx)
    other = (
        ctx[0]
        .post(
            "/api/auth/login", json={"email": "person@example.com", "password": "safe-password-123"}
        )
        .json()["token"]
    )
    assert ctx[0].post("/api/auth/logout", headers=first).status_code == 204
    assert ctx[0].get("/api/users/me", headers=first).status_code == 401
    assert (
        ctx[0].get("/api/users/me", headers={"Authorization": "Bearer " + other}).status_code == 200
    )
    assert ctx[0].post("/api/auth/logout").status_code == 401


def test_auth_budget_persists_without_addresses(ctx):
    ctx[1].state.settings.auth_rate_limit_enabled = True
    ctx[1].state.settings.auth_rate_limit_per_email = 2
    for _ in range(2):
        assert (
            ctx[0]
            .post("/api/auth/login", json={"email": "blocked@example.com", "password": "wrong"})
            .status_code
            == 401
        )
    response = ctx[0].post(
        "/api/auth/login", json={"email": "blocked@example.com", "password": "wrong"}
    )
    assert response.status_code == 429 and int(response.headers["Retry-After"]) > 0
    with ctx[1].state.engine.connect() as connection:
        keys = connection.scalars(text("SELECT key FROM identity_rate_limits")).all()
    assert keys and all(len(item) == 64 and "@" not in item for item in keys)


def test_business_notification_idempotency_and_privacy(ctx):
    headers = logged_in(ctx)
    uid = ctx[0].get("/api/users/me", headers=headers).json()["id"]
    data = {
        "sourceId": str(uuid4()),
        "userId": uid,
        "template": "ticket_delivery",
        "ticketId": str(uuid4()),
    }
    assert ctx[0].post("/internal/notifications", json=data, headers=headers).status_code == 401
    result = ctx[0].post("/internal/notifications", json=data, headers=internal(ctx))
    assert result.status_code == 202
    assert (
        ctx[0].post("/internal/notifications", json=data, headers=internal(ctx)).json()["id"]
        == result.json()["id"]
    )
    assert (
        ctx[0]
        .post(
            "/internal/notifications",
            json={**data, "ticketId": str(uuid4())},
            headers=internal(ctx),
        )
        .status_code
        == 409
    )
    assert (
        ctx[0]
        .post("/internal/notifications", json={**data, "simulation": False}, headers=internal(ctx))
        .status_code
        == 422
    )
    rows = ctx[0].get("/api/notifications", headers=headers).json()
    assert len(rows["items"]) == 1 and rows["items"][0]["link"].endswith(data["ticketId"])
    assert "encrypted" not in str(rows) and "token" not in str(rows)
    other = logged_in(ctx, "other@example.com")
    assert ctx[0].get("/api/notifications", headers=other).json()["items"] == []
    assert (
        ctx[0]
        .patch("/api/notifications/" + str(result.json()["id"]) + "/read", headers=other)
        .status_code
        == 404
    )
    assert (
        ctx[0]
        .patch("/api/notifications/" + str(result.json()["id"]) + "/read", headers=headers)
        .status_code
        == 204
    )
    assert ctx[0].get("/api/notifications", headers=headers).json()["items"][0]["readAt"]


def test_delivery_retry_and_skip_locked_workers(ctx):
    headers = logged_in(ctx)
    uid = UUID(ctx[0].get("/api/users/me", headers=headers).json()["id"])
    notifier = ctx[1].state.notifications
    with ctx[1].state.session_factory() as session:
        row = notifier.enqueue(
            session,
            uid,
            "order_confirmation",
            "business",
            uuid4(),
            {"orderId": str(uuid4()), "simulation": True},
        )
        session.commit()
        job_id = row.id
    delivered = []

    class Mailer:
        fail = True

        def send_notification(self, *args, **kwargs):
            if self.fail:
                raise OSError("sensitive provider failure")
            delivered.append((args, kwargs))

    mailer = Mailer()
    assert notifier.drain(ctx[1].state.session_factory, mailer, only_id=job_id) == 0
    with ctx[1].state.session_factory() as session:
        row = session.get(Notification, job_id)
        assert (
            row.state == "pending"
            and row.attempts == 1
            and row.failure_code == "delivery_unavailable"
        )
        row.next_attempt_at = datetime.now(UTC) - timedelta(seconds=1)
        session.commit()
    mailer.fail = False
    with ThreadPoolExecutor(2) as pool:
        counts = list(
            pool.map(
                lambda _: notifier.drain(ctx[1].state.session_factory, mailer, only_id=job_id),
                range(2),
            )
        )
    assert sum(counts) == 1 and len(delivered) == 1
    assert "Simulation only" in delivered[0][0][2]
    assert delivered[0][1]["message_id"] == f"<notification-{job_id}@biletflow>"


def test_expired_identity_job_never_delivers(ctx):
    from conftest import registered

    from app.models.identity import ActionToken

    ctx[1].state.settings.notification_eager_delivery = False
    registered(ctx, verify=False)
    with ctx[1].state.session_factory() as session:
        token = session.scalar(select(ActionToken))
        token.expires_at = datetime.now(UTC) - timedelta(seconds=1)
        session.commit()
    assert ctx[1].state.notifications.drain(ctx[1].state.session_factory, ctx[2]) == 0
    assert not ctx[2].messages
    with ctx[1].state.session_factory() as session:
        assert session.scalar(select(Notification)).state == "skipped"


def test_resend_verification_retries_queued_registration(ctx):
    ctx[1].state.settings.notification_eager_delivery = False
    result = ctx[0].post(
        "/api/auth/register", json={"email": "resend@example.com", "password": "safe-password-123"}
    )
    assert result.status_code == 201 and not ctx[2].messages
    assert (
        ctx[0]
        .post("/api/auth/resend-verification", json={"email": "unknown@example.com"})
        .status_code
        == 200
    )
    assert (
        ctx[0]
        .post("/api/auth/resend-verification", json={"email": "resend@example.com"})
        .status_code
        == 200
    )
    assert ctx[1].state.notifications.drain(ctx[1].state.session_factory, ctx[2]) == 2
    raw = ctx[2].messages[-1][2]
    assert ctx[0].post("/api/auth/verify-email", json={"token": raw}).status_code == 200


def test_platform_support_thread_assignment_notifications_and_privacy(ctx):
    requester, uid = role(ctx, "organizer@example.com", "organizer")
    admin, aid = role(ctx, "admin@example.com", "platform_admin")
    unrelated = logged_in(ctx, "stranger@example.com")
    payload = {
        "subject": "Activation question",
        "message": "How does the simulation work?",
        "category": "account",
    }
    req = key(requester)
    created = ctx[0].post("/api/support/cases", headers=req, json=payload)
    assert created.status_code == 201, created.text
    identity = created.json()["id"]
    path = "/api/support/cases/" + identity
    assert ctx[0].post("/api/support/cases", headers=req, json=payload).status_code == 200
    assert (
        ctx[0]
        .post("/api/support/cases", headers=req, json={**payload, "subject": "different"})
        .status_code
        == 409
    )
    assert ctx[0].get(path, headers=unrelated).status_code == 404
    assert ctx[0].get(path + "/messages", headers=unrelated).status_code == 404
    assert (
        ctx[0].patch(path, headers=key(requester), json={"status": "resolved"}).status_code == 403
    )
    changed = ctx[0].patch(
        path, headers=key(admin), json={"status": "in_progress", "assigneeId": str(aid)}
    )
    assert changed.status_code == 200, changed.text
    reply_key = key(admin)
    reply = ctx[0].post(path + "/messages", headers=reply_key, json={"body": "It is a demo only."})
    assert reply.status_code == 201, reply.text
    assert (
        ctx[0]
        .post(path + "/messages", headers=reply_key, json={"body": "It is a demo only."})
        .status_code
        == 200
    )
    messages = ctx[0].get(path + "/messages", headers=requester, params={"after": 1}).json()
    assert len(messages["items"]) == 1 and messages["nextCursor"] == 2
    assert len(ctx[0].get(path + "/history", headers=requester).json()["items"]) == 3
    notices = ctx[0].get("/api/notifications", headers=requester).json()["items"]
    assert {row["template"] for row in notices} == {"support_assignment", "support_reply"}
    assert ctx[0].patch(path, headers=key(admin), json={"status": "resolved"}).status_code == 200
    assert (
        ctx[0]
        .post(path + "/messages", headers=key(requester), json={"body": "One more question"})
        .status_code
        == 201
    )
    assert ctx[0].get(path, headers=requester).json()["status"] == "open"
    assert ctx[0].get("/api/support/cases", headers=unrelated).json()["items"] == []


def test_event_support_requires_authoritative_context_and_staff(ctx):
    owner, oid = role(ctx, "owner@example.com", "organizer")
    attendee = logged_in(ctx, "attendee@example.com")
    other, _ = role(ctx, "other@example.com", "organizer")
    event = context_fixture(ctx, oid)
    payload = {
        "subject": "Venue question",
        "message": "Where is entry?",
        "category": "event_information",
        "eventId": str(event),
    }
    response = ctx[0].post("/api/support/cases", headers=key(attendee), json=payload)
    assert response.status_code == 201, response.text
    path = "/api/support/cases/" + response.json()["id"]
    assert ctx[0].get(path, headers=owner).status_code == 200
    assert ctx[0].get(path, headers=other).status_code == 404
    assert (
        ctx[0].post(path + "/messages", headers=key(other), json={"body": "no"}).status_code == 404
    )
    assert (
        ctx[0]
        .post(path + "/messages", headers=key(owner), json={"body": "Main entrance"})
        .status_code
        == 201
    )
    assert (
        ctx[0]
        .post(
            "/api/support/cases",
            headers=key(attendee),
            json={k: v for k, v in payload.items() if k != "eventId"},
        )
        .status_code
        == 422
    )
    assert ctx[0].patch(path, headers=key(owner), json={"escalated": True}).status_code == 200
    assert ctx[0].get(path, headers=owner).status_code == 404


def test_unassigned_event_admin_has_no_global_support_access(ctx):
    requester = logged_in(ctx, "requester@example.com")
    owner, oid = role(ctx, "owner@example.com", "organizer")
    staff, sid = role(ctx, "staff@example.com", "event_admin")
    event = context_fixture(ctx, oid)
    response = ctx[0].post(
        "/api/support/cases",
        headers=key(requester),
        json={
            "subject": "Entry",
            "message": "Question",
            "category": "check_in",
            "eventId": str(event),
        },
    )
    path = "/api/support/cases/" + response.json()["id"]
    assert ctx[0].get(path, headers=staff).status_code == 404
    context_fixture(ctx, oid, event, staff=sid)
    assert ctx[0].get(path, headers=staff).status_code == 200
    assert ctx[0].patch(path, headers=key(staff), json={"assigneeId": str(sid)}).status_code == 200


@pytest.mark.parametrize("code", [404, 500, 302])
def test_unavailable_business_context_fails_closed(ctx, code):
    attendee = logged_in(ctx)
    ctx[1].state.business.client.close()
    ctx[1].state.business.client = httpx.Client(
        base_url="http://fixture", transport=httpx.MockTransport(lambda _: httpx.Response(code))
    )
    response = ctx[0].post(
        "/api/support/cases",
        headers=key(attendee),
        json={
            "subject": "Test",
            "message": "Test",
            "category": "technical",
            "eventId": str(uuid4()),
        },
    )
    assert response.status_code == (404 if code == 404 else 503)
    with ctx[1].state.session_factory() as session:
        assert session.scalar(select(SupportCase)) is None


def test_admin_settings_suspension_audit_and_stale_jwt(ctx):
    admin, aid = role(ctx, "admin@example.com", "platform_admin")
    target = logged_in(ctx, "target@example.com")
    tid = ctx[0].get("/api/users/me", headers=target).json()["id"]
    assert ctx[0].get("/api/admin/users", headers=target).status_code == 403
    assert ctx[0].get("/api/admin/platform-settings", headers=target).status_code == 403
    current = ctx[0].get("/api/admin/platform-settings", headers=admin).json()
    payload = {"activationFeeMinor": 123400, "expectedVersion": current["version"]}
    response = ctx[0].patch("/api/admin/platform-settings", headers=admin, json=payload)
    assert response.status_code == 200, response.text
    assert response.json()["currency"] == "KZT" and response.json()["simulationOnly"] is True
    assert (
        ctx[0].patch("/api/admin/platform-settings", headers=admin, json=payload).status_code == 409
    )
    assert (
        ctx[0]
        .get("/internal/platform-settings", headers=internal(ctx))
        .json()["activationFeeMinor"]
        == 123400
    )
    assert ctx[0].get("/internal/platform-settings", headers=admin).status_code == 401
    assert (
        ctx[0].patch("/api/admin/users/" + tid, headers=admin, json={"isActive": False}).status_code
        == 200
    )
    assert ctx[0].get("/api/users/me", headers=target).status_code == 401
    assert (
        ctx[0]
        .patch("/api/admin/users/" + str(aid), headers=admin, json={"isActive": False})
        .status_code
        == 409
    )
    assert (
        ctx[0].patch("/api/admin/users/" + tid, headers=admin, json={"isActive": True}).status_code
        == 200
    )
    assert ctx[0].get("/api/users/me", headers=target).status_code == 401
    audit = ctx[0].get("/api/admin/audit", headers=admin).json()["items"]
    assert len(audit) == 3 and all("password" not in str(item) for item in audit)
    assert (
        len(ctx[0].get("/api/admin/users", headers=admin, params={"q": "target@"}).json()["items"])
        == 1
    )


def test_concurrent_support_message_retries_do_not_duplicate(ctx):
    requester, _ = role(ctx, "org@example.com", "organizer")
    response = ctx[0].post(
        "/api/support/cases",
        headers=key(requester),
        json={"subject": "Help", "message": "First", "category": "technical"},
    )
    path = "/api/support/cases/" + response.json()["id"] + "/messages"
    request_key = key(requester)
    with ThreadPoolExecutor(2) as pool:
        statuses = list(
            pool.map(
                lambda _: (
                    ctx[0].post(path, headers=request_key, json={"body": "Retry"}).status_code
                ),
                range(2),
            )
        )
    assert sorted(statuses) == [200, 201]
    assert len(ctx[0].get(path, headers=requester).json()["items"]) == 2


def test_request_id_is_bounded_and_propagates(ctx):
    identity = str(uuid4())
    for incoming in [identity, identity.replace("-", "")]:
        assert (
            ctx[0].get("/health", headers={"X-Request-ID": incoming}).headers["X-Request-ID"]
            == identity
        )
    assert (
        ctx[0].get("/health", headers={"X-Request-ID": "invalid"}).headers["X-Request-ID"]
        != "invalid"
    )


def test_worker_recovers_an_expired_lease(ctx):
    headers = logged_in(ctx)
    user = UUID(ctx[0].get("/api/users/me", headers=headers).json()["id"])
    with ctx[1].state.session_factory() as session:
        row = ctx[1].state.notifications.enqueue(
            session, user, "event_updated", "business", uuid4(), {"eventId": str(uuid4())}
        )
        row.state = "sending"
        row.lease_id = uuid4()
        row.lease_until = datetime.now(UTC) - timedelta(seconds=1)
        session.commit()
        identity = row.id
    assert (
        ctx[1].state.notifications.drain(ctx[1].state.session_factory, ctx[2], only_id=identity)
        == 1
    )
    with ctx[1].state.session_factory() as session:
        row = session.get(Notification, identity)
        assert row.state == "delivered" and row.lease_id is None


def test_ordinary_roles_cannot_read_private_platform_data(ctx):
    for name in ["attendee", "organizer", "event_admin"]:
        headers, _ = role(ctx, name + "@example.com", name)
        for path in [
            "/api/admin/users",
            "/api/admin/audit",
            "/api/admin/notification-jobs",
            "/api/admin/platform-settings",
            "/api/admin/business/orders",
        ]:
            assert ctx[0].get(path, headers=headers).status_code == 403


def test_business_adapter_hides_redirects_malformed_and_oversized_bodies():
    client = BusinessClient("http://example.test")
    try:
        for response in [
            httpx.Response(302, headers={"Location": "http://evil.test"}),
            httpx.Response(200, content=b"not-json"),
            httpx.Response(200, content=b"x" * 262145),
        ]:
            client.client.close()
            client.client = httpx.Client(
                base_url="http://example.test",
                transport=httpx.MockTransport(lambda _, response=response: response),
            )
            from fastapi import HTTPException

            with pytest.raises(HTTPException) as caught:
                client.get("/api/events/" + str(uuid4()), "Bearer controlled-test", "request-id")
            assert caught.value.status_code == 503
    finally:
        client.close()


@pytest.mark.parametrize(
    "mutation", [None, "wrong_event", "wrong_order", "wrong_user", "bad_resource"]
)
def test_order_ticket_support_context_is_consistent_and_private(ctx, mutation):
    headers = logged_in(ctx)
    actor = ctx[0].get("/api/users/me", headers=headers).json()["id"]
    _, owner = role(ctx, "context-owner@example.com", "organizer")
    event, order, ticket = uuid4(), uuid4(), uuid4()
    records = {
        "/api/orders/" + str(order): {
            "id": str(order),
            "eventId": str(event),
            "organizerId": str(owner),
            "userId": actor,
        },
        "/api/tickets/" + str(ticket): {
            "id": str(ticket),
            "eventId": str(event),
            "organizerId": str(owner),
            "attendeeId": actor,
            "orderId": str(order),
        },
    }
    entry = records["/api/tickets/" + str(ticket)]
    if mutation == "wrong_event":
        entry["eventId"] = str(uuid4())
    if mutation == "wrong_order":
        entry["orderId"] = str(uuid4())
    if mutation == "wrong_user":
        entry["attendeeId"] = str(uuid4())
    if mutation == "bad_resource":
        entry["id"] = "malformed"
    ctx[1].state.business.client.close()
    ctx[1].state.business.client = httpx.Client(
        base_url="http://fixture",
        transport=httpx.MockTransport(
            lambda request: (
                httpx.Response(200, json=records[request.url.path])
                if request.url.path in records
                else httpx.Response(404)
            )
        ),
    )
    response = ctx[0].post(
        "/api/support/cases",
        headers=key(headers),
        json={
            "subject": "Private ticket",
            "message": "Refund question",
            "category": "refund",
            "eventId": str(event),
            "orderId": str(order),
            "ticketId": str(ticket),
        },
    )
    expected = {
        None: 201,
        "wrong_event": 422,
        "wrong_order": 422,
        "wrong_user": 403,
        "bad_resource": 503,
    }[mutation]
    assert response.status_code == expected, response.status_code


def test_internal_credentials_reject_non_ascii_without_500(ctx):
    response = ctx[0].get(
        "/internal/platform-settings", headers=[(b"Authorization", b"Bearer \xff")]
    )
    assert response.status_code == 401


def png(width=20, height=12):
    import io

    from PIL import Image, PngImagePlugin

    stream = io.BytesIO()
    metadata = PngImagePlugin.PngInfo()
    metadata.add_text("private", "must disappear")
    Image.new("RGB", (width, height), "red").save(stream, "PNG", pnginfo=metadata)
    return stream.getvalue()


def test_event_image_normalization_ownership_replay_and_visibility(ctx):
    import io

    from PIL import Image

    from app.models.platform import AdminAudit

    owner, uid = role(ctx, "image-owner@example.com", "organizer")
    event = context_fixture(ctx, uid)
    path = "/api/event-images/" + str(event)
    other, _ = role(ctx, "image-other@example.com", "organizer")
    attendee = logged_in(ctx)
    assert (
        ctx[0]
        .put(path, headers={**attendee, "Content-Type": "image/png"}, content=png())
        .status_code
        == 403
    )
    assert (
        ctx[0].put(path, headers={**other, "Content-Type": "image/png"}, content=png()).status_code
        == 403
    )
    request = {**owner, "Content-Type": "image/png"}
    response = ctx[0].put(path, headers=request, content=png())
    assert response.status_code == 200
    assert response.json()["width"] == 20 and response.json()["height"] == 12
    assert ctx[0].put(path, headers=request, content=png()).json() == response.json()
    downloaded = ctx[0].get(path)
    assert downloaded.status_code == 200 and downloaded.headers["Content-Type"] == "image/webp"
    with Image.open(io.BytesIO(downloaded.content)) as decoded:
        assert decoded.size == (20, 12) and "private" not in decoded.info
    with ctx[1].state.session_factory() as session:
        assert (
            len(session.scalars(select(AdminAudit).where(AdminAudit.action == "event_image")).all())
            == 1
        )
    ctx[1].state.business.client.close()
    ctx[1].state.business.client = httpx.Client(
        base_url="http://fixture", transport=httpx.MockTransport(lambda _: httpx.Response(404))
    )
    assert ctx[0].get(path).status_code == 404


@pytest.mark.parametrize(
    "mime,body,status",
    [
        ("image/svg+xml", b"<svg/>", 415),
        ("image/png", b"bad bytes", 422),
        ("image/jpeg", png(), 422),
        ("image/png", png(4097, 1), 422),
        ("image/png", b"x" * 1048577, 413),
    ],
)
def test_event_image_rejects_unsafe_inputs(ctx, mime, body, status):
    headers, owner = role(ctx, "image-validation@example.com", "organizer")
    event = context_fixture(ctx, owner)
    response = ctx[0].put(
        "/api/event-images/" + str(event), headers={**headers, "Content-Type": mime}, content=body
    )
    assert response.status_code == status
    assert not list(ctx[1].state.settings.image_storage_dir.glob("*.webp"))
