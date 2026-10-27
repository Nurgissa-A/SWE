"""Real Python -> RS256 -> Go ownership/authorization smoke on Compose services."""

import os
import re
import time
from pathlib import Path

import httpx
import jwt
from cryptography.hazmat.primitives.asymmetric import rsa
from smoke_identity import onboard


def main():
    with httpx.Client(base_url=os.getenv("IDENTITY_URL", "http://localhost:8000")) as identity:
        with httpx.Client(base_url=os.getenv("EVENT_URL", "http://localhost:8080")) as events:
            with httpx.Client(
                base_url=os.getenv("MAILPIT_URL", "http://localhost:8025")
            ) as mailbox:
                first, user = onboard(identity, mailbox, "organizer-interop")
                second, other = onboard(identity, mailbox, "other-interop")
                for headers, record in [(first, user), (second, other)]:
                    assert (
                        identity.post(
                            "/api/organizer-profile",
                            headers=headers,
                            json={"contactEmail": record["email"]},
                        ).status_code
                        == 201
                    )
                attendee, _ = onboard(identity, mailbox, "attendee-interop")
                data = {
                    "title": "JWT integration event",
                    "startsAt": "2030-01-01T10:00:00Z",
                }
                assert events.post("/api/events", json=data).status_code == 401
                assert events.post("/api/events", json=data, headers=attendee).status_code == 403
                result = events.post("/api/events", json=data, headers=first)
                assert result.status_code == 201, result.status_code
                event = result.json()
                assert event["organizerId"] == user["id"]
                path = "/api/events/" + event["id"]
                assert events.get(path).status_code == 404
                assert events.get(path, headers=second).status_code == 404
                assert events.post(path + "/publish", headers=second).status_code == 403
                assert (
                    events.post(
                        "/api/events",
                        headers=first,
                        json={**data, "organizerId": other["id"]},
                    ).status_code
                    == 400
                )
                good = first["Authorization"].split()[1]
                # Derive controlled negative fixtures from a locally issued smoke token.
                claims = jwt.decode(good, options={"verify_signature": False})
                kid = jwt.get_unverified_header(good)["kid"]
                private = Path(
                    os.getenv("JWT_PRIVATE_KEY_FILE", ".secrets/jwt-private.pem")
                ).read_bytes()
                expired = jwt.encode(
                    {**claims, "exp": int(time.time()) - 60},
                    private,
                    algorithm="RS256",
                    headers={"kid": kid},
                )
                wrong = jwt.encode(
                    claims,
                    rsa.generate_private_key(public_exponent=65537, key_size=2048),
                    algorithm="RS256",
                    headers={"kid": kid},
                )
                for bad in [expired, wrong, "malformed"]:
                    assert (
                        events.post(
                            "/api/events",
                            json=data,
                            headers={"Authorization": "Bearer " + bad},
                        ).status_code
                        == 401
                    )
                assert events.post(path + "/publish", headers=first).status_code == 200
                assert events.get(path).status_code == 200
                visible = events.get("/api/events", headers=attendee)
                assert visible.status_code == 200 and event["id"] in [
                    x["id"] for x in visible.json()
                ]
                identity.post(
                    "/api/auth/request-password-reset", json={"email": user["email"]}
                ).raise_for_status()
                reset = None
                for message in mailbox.get("/api/v1/messages").json()["messages"]:
                    if any(to.get("Address") == user["email"] for to in message.get("To", [])):
                        body = mailbox.get("/api/v1/message/" + message["ID"]).json()["Text"]
                        if "/reset-password?" in body:
                            reset = re.search(r"token=([A-Za-z0-9_-]+)", body).group(1)
                            break
                assert reset, "Reset email not delivered"
                identity.post(
                    "/api/auth/reset-password",
                    json={"token": reset, "newPassword": "local-smoke-reset-123"},
                ).raise_for_status()
                assert events.post("/api/events", json=data, headers=first).status_code == 401
    print(
        "Python-Go HTTP integration passed: RS256, owner UUID, roles, draft isolation, "
        "foreign-owner denial, forged input, invalid/expired signatures, "
        "publish and reset revocation."
    )


if __name__ == "__main__":
    main()
