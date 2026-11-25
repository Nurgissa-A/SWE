from conftest import logged_in
from sqlalchemy import select

from app.models.identity import Role, User


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
