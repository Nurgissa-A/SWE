from conftest import logged_in
from sqlalchemy import select

from app.models.identity import Role, User

PROFILE = {
    "contactEmail": "organizer@example.com",
    "contactPhone": "+7 777 123 4567",
    "payoutAccountPlaceholder": "demo:account-1",
}


def test_onboarding_and_update(ctx):
    headers = logged_in(ctx)
    assert ctx[0].get("/api/organizer-profile", headers=headers).status_code == 403
    response = ctx[0].post("/api/organizer-profile", headers=headers, json=PROFILE)
    assert response.status_code == 201
    uid = ctx[0].get("/api/users/me", headers=headers).json()["id"]
    assert response.json()["userId"] == uid
    assert ctx[0].get("/api/users/me", headers=headers).json()["role"] == "organizer"
    assert ctx[0].post("/api/organizer-profile", headers=headers, json=PROFILE).status_code == 409
    assert (
        ctx[0].get("/api/organizer-profile", headers=headers).json()["id"] == response.json()["id"]
    )
    result = ctx[0].patch(
        "/api/organizer-profile", headers=headers, json={**PROFILE, "contactPhone": None}
    )
    assert result.status_code == 200 and result.json()["contactPhone"] is None


def test_profile_ownership_and_input_restrictions(ctx):
    first = logged_in(ctx, "first@example.com")
    second = logged_in(ctx, "second@example.com")
    ctx[0].post("/api/organizer-profile", headers=first, json=PROFILE)
    assert ctx[0].get("/api/organizer-profile", headers=second).status_code == 403
    assert (
        ctx[0]
        .post("/api/organizer-profile", headers=second, json={**PROFILE, "userId": "forged"})
        .status_code
        == 422
    )
    assert (
        ctx[0]
        .post(
            "/api/organizer-profile",
            headers=second,
            json={**PROFILE, "payoutAccountPlaceholder": "real-bank-account"},
        )
        .status_code
        == 422
    )
    assert (
        ctx[0]
        .post(
            "/api/organizer-profile", headers=second, json={**PROFILE, "contactPhone": "<script>"}
        )
        .status_code
        == 422
    )


def test_event_admin_and_platform_admin_cannot_self_onboard(ctx):
    headers = logged_in(ctx)
    for role in [Role.EVENT_ADMIN, Role.PLATFORM_ADMIN]:
        with ctx[1].state.session_factory() as session:
            row = session.scalar(select(User))
            row.role = role.value
            session.commit()
        assert (
            ctx[0].post("/api/organizer-profile", headers=headers, json=PROFILE).status_code == 403
        )
