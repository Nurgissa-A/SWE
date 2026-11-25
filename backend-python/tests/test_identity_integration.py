from io import BytesIO
from uuid import UUID, uuid4

import httpx
from conftest import logged_in
from PIL import Image
from sqlalchemy import select

from app.models.identity import AccountAudit, Role, User


def test_staff_lookup_user_admin_and_account_locale(ctx):
    client, app, _ = ctx
    staff = logged_in(ctx, "staff@example.com")
    organizer = logged_in(ctx, "organizer@example.com")
    admin = logged_in(ctx, "admin@example.com")
    with app.state.session_factory.begin() as session:
        for email, role in [
            ("organizer@example.com", Role.ORGANIZER),
            ("admin@example.com", Role.PLATFORM_ADMIN),
        ]:
            session.scalar(select(User).where(User.email == email)).role = role

    assert client.patch("/api/users/me", headers=staff, json={"locale": "ru"}).status_code == 200
    assert client.get("/api/users/me", headers=staff).json()["locale"] == "ru"
    assert client.patch("/api/users/me", headers=staff, json={"locale": "fr"}).status_code == 422
    lookup = "/api/users/lookup?email=STAFF@example.com"
    assert client.get(lookup, headers=staff).status_code == 403
    found = client.get(lookup, headers=organizer)
    assert found.status_code == 200 and found.json()["locale"] == "ru"
    assert client.get("/api/admin/users", headers=organizer).status_code == 403
    matches = client.get("/api/admin/users?q=staff", headers=admin).json()
    assert [user["email"] for user in matches] == ["staff@example.com"]

    path = "/api/admin/users/" + found.json()["id"]
    suspended = client.patch(path, headers=admin, json={"isActive": False})
    assert suspended.status_code == 200 and suspended.json()["isActive"] is False
    assert client.get("/api/users/me", headers=staff).status_code == 401
    assert client.get(lookup, headers=organizer).status_code == 404
    assert client.patch(path, headers=admin, json={"isActive": True}).status_code == 200
    assert client.get("/api/users/me", headers=staff).status_code == 401
    with app.state.session_factory() as session:
        entries = session.scalars(
            select(AccountAudit).where(AccountAudit.account_id == UUID(found.json()["id"]))
        ).all()
        assert sorted(entry.is_active for entry in entries) == [False, True]


def test_logout_revokes_only_the_current_session(ctx):
    client, _, _ = ctx
    first = logged_in(ctx)
    second = client.post(
        "/api/auth/login", json={"email": "person@example.com", "password": "safe-password-123"}
    ).json()["token"]
    assert client.post("/api/auth/logout", headers=first).status_code == 204
    assert client.get("/api/users/me", headers=first).status_code == 401
    assert (
        client.get("/api/users/me", headers={"Authorization": f"Bearer {second}"}).status_code
        == 200
    )


def test_event_cover_is_normalized_and_obeys_go_visibility(ctx, monkeypatch):
    client, app, _ = ctx
    owner_headers = logged_in(ctx)
    owner = client.get("/api/users/me", headers=owner_headers).json()["id"]
    with app.state.session_factory.begin() as session:
        session.scalar(select(User)).role = Role.ORGANIZER
    public = False
    event_id = uuid4()

    def lookup(path, *, headers):
        assert path == f"/api/events/{event_id}"
        if headers == owner_headers or public:
            return httpx.Response(
                200,
                json={"id": str(event_id), "organizerId": owner, "visibility": "public"},
                request=httpx.Request("GET", "http://go" + path),
            )
        return httpx.Response(404)

    monkeypatch.setattr(app.state.events, "get", lookup)
    path = f"/api/event-covers/{event_id}"
    image = BytesIO()
    Image.new("RGB", (32, 16)).save(image, format="PNG")
    upload_headers = {**owner_headers, "Content-Type": "image/png"}
    assert client.put(path, headers=upload_headers, content=b"not an image").status_code == 422
    uploaded = client.put(path, headers=upload_headers, content=image.getvalue())
    assert uploaded.status_code == 200 and uploaded.json()["imageUrl"] == path
    assert client.get(path).status_code == 404
    public = True
    downloaded = client.get(path)
    assert downloaded.status_code == 200 and downloaded.headers["content-type"] == "image/webp"
    assert Image.open(BytesIO(downloaded.content)).size == (32, 16)
