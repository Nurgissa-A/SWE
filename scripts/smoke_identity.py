"""Exercise real SMTP + identity HTTP; prints no passwords, bearer or action tokens."""

import os
import re
import secrets
import time
from uuid import uuid4

import httpx


def onboard(client, mailbox, name):
    email = f"{name}-{uuid4().hex}@example.com"
    password = secrets.token_urlsafe(24)
    result = client.post(
        "/api/auth/register", json={"email": email, "password": password}
    )
    assert result.status_code == 201, result.status_code
    token = None
    for _ in range(30):
        for message in mailbox.get("/api/v1/messages").json().get("messages", []):
            if any(to.get("Address") == email for to in message.get("To", [])):
                body = mailbox.get("/api/v1/message/" + message["ID"]).json()["Text"]
                token = re.search(r"token=([A-Za-z0-9_-]+)", body).group(1)
                break
        if token:
            break
        time.sleep(0.2)
    assert token, "Verification email not captured"
    assert (
        client.post("/api/auth/verify-email", json={"token": token}).status_code == 200
    )
    assert (
        client.post("/api/auth/verify-email", json={"token": token}).status_code == 400
    )
    login = client.post("/api/auth/login", json={"email": email, "password": password})
    assert login.status_code == 200
    return {"Authorization": "Bearer " + login.json()["token"]}, login.json()["user"]


def main():
    with httpx.Client(
        base_url=os.getenv("IDENTITY_URL", "http://localhost:8000")
    ) as api, httpx.Client(
        base_url=os.getenv("MAILPIT_URL", "http://localhost:8025")
    ) as mailbox:
        for route in [
            "/health",
            "/ready",
            "/docs",
            "/redoc",
            "/openapi.json",
            "/.well-known/jwks.json",
        ]:
            assert api.get(route).status_code == 200, route
        headers, user = onboard(api, mailbox, "identity-smoke")
        assert api.get("/api/users/me", headers=headers).json()["id"] == user["id"]
        assert api.get("/api/users/me").status_code == 401
        profile = api.post(
            "/api/organizer-profile",
            headers=headers,
            json={"contactEmail": user["email"]},
        )
        assert profile.status_code == 201 and profile.json()["userId"] == user["id"]
        assert (
            api.get("/api/users/me", headers=headers).json()["role"] == "organizer"
        )
    print(
        "Identity SMTP/HTTP smoke passed: docs, readiness, register, real email, verify/reuse, login, me, RBAC and organizer profile."
    )


if __name__ == "__main__":
    main()
