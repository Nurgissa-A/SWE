"""Real HTTPS/SMTP/Go event integration. Go order/ticket producer is not simulated as completed."""

import io
import os
import secrets
import ssl
import subprocess
from pathlib import Path
from uuid import uuid4

import httpx
from PIL import Image
from smoke_identity import onboard

ROOT = Path(__file__).resolve().parents[1]


class ContainerMailbox:
    """Read only the private demo mail sink through Compose, never publish it via HTTPS."""

    def get(self, path):
        script = (
            "import httpx,json,sys; "
            "r=httpx.get('http://mailpit:8025'+sys.argv[1]); "
            "r.raise_for_status(); sys.stdout.write(r.text)"
        )
        result = subprocess.run(
            ["docker", "compose", "exec", "-T", "python", "python", "-c", script, path],
            cwd=ROOT,
            check=True,
            capture_output=True,
        )
        return httpx.Response(200, content=result.stdout)


def idem(headers):
    return {**headers, "Idempotency-Key": str(uuid4())}


def main():
    context = ssl.create_default_context(cafile=ROOT / ".secrets/tls/ca.crt")
    mailbox = ContainerMailbox()
    with httpx.Client(
        base_url=os.getenv("PLATFORM_URL", "https://localhost:8443"), verify=context, timeout=15
    ) as api:
        for path in ["/ready", "/health/go", "/login", "/docs", "/redoc", "/.well-known/jwks.json"]:
            assert api.get(path).status_code == 200, path
        assert api.get("/internal/platform-settings").status_code == 404
        organizer, user = onboard(api, mailbox, "platform-organizer")
        assert (
            api.post(
                "/api/organizer-profile", headers=organizer, json={"contactEmail": user["email"]}
            ).status_code
            == 201
        )
        password = secrets.token_urlsafe(24)
        admin, record = onboard(api, mailbox, "platform-admin", password=password)
        subprocess.run(
            [
                "docker",
                "compose",
                "exec",
                "-T",
                "python",
                "python",
                "-m",
                "app.cli",
                "set-role",
                record["email"],
                "--role",
                "platform_admin",
            ],
            cwd=ROOT,
            check=True,
            capture_output=True,
        )
        result = api.post("/api/auth/login", json={"email": record["email"], "password": password})
        assert result.status_code == 200
        admin = {"Authorization": "Bearer " + result.json()["token"]}
        attendee, attendee_user = onboard(api, mailbox, "platform-attendee")
        assert api.get("/api/admin/users", headers=attendee).status_code == 403
        settings = api.get("/api/admin/platform-settings", headers=admin).json()
        changed = api.patch(
            "/api/admin/platform-settings",
            headers=admin,
            json={
                "activationFeeMinor": settings["activationFeeMinor"],
                "expectedVersion": settings["version"],
            },
        )
        assert changed.status_code == 200 and changed.json()["simulationOnly"] is True
        request = idem(organizer)
        data = {
            "subject": "Platform HTTPS smoke",
            "message": "Confirm support workflow",
            "category": "technical",
        }
        case = api.post("/api/support/cases", headers=request, json=data)
        assert case.status_code == 201, case.status_code
        assert api.post("/api/support/cases", headers=request, json=data).status_code == 200
        path = "/api/support/cases/" + case.json()["id"]
        assert api.get(path, headers=attendee).status_code == 404
        assert (
            api.patch(
                path,
                headers=idem(admin),
                json={"status": "in_progress", "assigneeId": record["id"]},
            ).status_code
            == 200
        )
        assert (
            api.post(
                path + "/messages", headers=idem(admin), json={"body": "HTTPS support response"}
            ).status_code
            == 201
        )
        messages = api.get(path + "/messages", headers=organizer).json()
        assert len(messages["items"]) == 2 and messages["nextCursor"] == 2
        assert api.get("/api/notifications", headers=organizer).json()["items"]
        event = api.post(
            "/api/events",
            headers=organizer,
            json={
                "title": "Platform event support smoke",
                "startsAt": "2030-01-01T10:00:00Z",
                "venueId": str(uuid4()),
            },
        )
        assert event.status_code == 201
        identity = event.json()["id"]
        image = io.BytesIO()
        Image.new("RGB", (24, 16), "blue").save(image, "PNG")
        image_path = "/api/event-images/" + identity
        upload = api.put(
            image_path, headers={**organizer, "Content-Type": "image/png"}, content=image.getvalue()
        )
        assert upload.status_code == 200, upload.status_code
        assert (
            api.put(
                image_path,
                headers={**attendee, "Content-Type": "image/png"},
                content=image.getvalue(),
            ).status_code
            == 403
        )
        assert (
            api.post("/api/events/" + identity + "/publish", headers=organizer).status_code == 200
        )
        downloaded = api.get(image_path)
        assert downloaded.status_code == 200 and downloaded.headers["Content-Type"] == "image/webp"
        subprocess.run(
            ["docker", "compose", "restart", "python"], cwd=ROOT, check=True, capture_output=True
        )
        import time

        for _ in range(30):
            try:
                if api.get("/ready").status_code == 200:
                    break
            except httpx.HTTPError:
                pass
            time.sleep(1)
        assert api.get(image_path).content == downloaded.content
        event_case = api.post(
            "/api/support/cases",
            headers=idem(attendee),
            json={
                "subject": "Event entry",
                "message": "Which entrance?",
                "category": "event_information",
                "eventId": identity,
            },
        )
        assert event_case.status_code == 201, event_case.status_code
        assert (
            api.post(
                "/api/support/cases/" + event_case.json()["id"] + "/messages",
                headers=idem(organizer),
                json={"body": "Main entrance"},
            ).status_code
            == 201
        )
        assert (
            api.patch(
                "/api/admin/users/" + attendee_user["id"], headers=admin, json={"isActive": False}
            ).status_code
            == 200
        )
        assert api.get("/api/users/me", headers=attendee).status_code == 401
        assert api.get("/api/events", headers=attendee).status_code == 401
        assert api.post("/api/auth/logout", headers=organizer).status_code == 204
        assert api.get("/api/users/me", headers=organizer).status_code == 401
        assert api.get("/api/events", headers=organizer).status_code == 401
        assert api.get("/api/admin/audit", headers=admin).json()["items"]
    print(
        "HTTPS platform smoke passed: CA verification, private internal API, real SMTP onboarding, "
        "Platform Admin settings/suspension/audit, idempotent support/replies/notifications, "
        "real Go event-context ownership, image normalization/persistence and Go logout/revocation."
    )


if __name__ == "__main__":
    main()
