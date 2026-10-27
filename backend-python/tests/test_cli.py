import sys

import pytest
from conftest import logged_in

from app.cli import main


def test_operator_role_assignment_revokes_old_token(ctx, monkeypatch, database_url):
    headers = logged_in(ctx)
    monkeypatch.setenv("DATABASE_URL", database_url)
    monkeypatch.setattr(
        sys, "argv", ["identity", "set-role", "person@example.com", "--role", "event_admin"]
    )
    main()
    assert ctx[0].get("/api/users/me", headers=headers).status_code == 401
    response = ctx[0].post(
        "/api/auth/login", json={"email": "person@example.com", "password": "safe-password-123"}
    )
    assert response.json()["user"]["role"] == "event_admin"
    monkeypatch.setattr(sys, "argv", ["identity", "disable-user", "person@example.com"])
    main()
    assert (
        ctx[0]
        .get("/api/users/me", headers={"Authorization": "Bearer " + response.json()["token"]})
        .status_code
        == 401
    )


def test_operator_rejects_unknown_user(ctx, monkeypatch, database_url):
    monkeypatch.setenv("DATABASE_URL", database_url)
    monkeypatch.setattr(
        sys, "argv", ["identity", "set-role", "absent@example.com", "--role", "platform_admin"]
    )
    with pytest.raises(SystemExit):
        main()
