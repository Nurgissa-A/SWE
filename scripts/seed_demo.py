"""Load demonstration data through the real APIs (no direct database writes).

Run after the services are up. Accounts that do not exist yet are registered and verified
through Mailpit. Administrator roles are never self-assigned, so an operator then promotes the
demo admin account (see README):

    python3 scripts/seed_demo.py
    docker compose exec python python -m app.cli set-role admin@biletflow.kz --role platform_admin

All demo accounts use the password demo12345. Payments use the simulator's test cards.
The script is idempotent: an existing demo event is left as it is.
"""

import os
import re
import sys
import time
from datetime import UTC, datetime, timedelta

import httpx

PASSWORD = os.getenv("DEMO_PASSWORD", "demo12345")
IDENTITY = os.getenv("IDENTITY_URL", "http://localhost:8000")
EVENTS = os.getenv("EVENT_URL", "http://localhost:8080")
MAILPIT = os.getenv("MAILPIT_URL", "http://localhost:8025")
CARD = "4242 4242 4242 4242"


def at(days, hour, minute=0):
    """A time `days` from today at hour:minute in Almaty (UTC+5), as ISO 8601 UTC."""
    local = datetime.now(UTC).replace(hour=0, minute=0, second=0, microsecond=0) + timedelta(
        days=days
    )
    return (local + timedelta(hours=hour - 5, minutes=minute)).isoformat().replace("+00:00", "Z")


def account(identity, mailbox, email, name, locale):
    login = identity.post("/api/auth/login", json={"email": email, "password": PASSWORD})
    if login.status_code == 401 and "verify" not in login.text.lower():
        reg = identity.post("/api/auth/register", json={"email": email, "password": PASSWORD})
        if reg.status_code not in (201, 409):
            sys.exit(f"Could not register {email}: {reg.status_code}")
        token = None
        for _ in range(50):
            for message in mailbox.get("/api/v1/messages").json().get("messages", []):
                if any(to.get("Address") == email for to in message.get("To", [])):
                    body = mailbox.get("/api/v1/message/" + message["ID"]).json()["Text"]
                    found = re.search(r"token=([A-Za-z0-9_-]+)", body)
                    if found:
                        token = found.group(1)
                        break
            if token:
                break
            time.sleep(0.2)
        if not token:
            sys.exit(f"No verification e-mail for {email}; is Mailpit running?")
        identity.post("/api/auth/verify-email", json={"token": token})
        login = identity.post("/api/auth/login", json={"email": email, "password": PASSWORD})
    if login.status_code != 200:
        sys.exit(f"Cannot sign in as {email}: {login.status_code} {login.text[:200]}")
    headers = {"Authorization": "Bearer " + login.json()["token"]}
    identity.patch("/api/users/me", headers=headers, json={"displayName": name, "locale": locale})
    return headers, login.json()["user"]


def ok(response, *codes):
    if response.status_code not in (codes or (200, 201, 204)):
        sys.exit(
            f"{response.request.method} {response.request.url.path}: "
            f"{response.status_code} {response.text[:300]}"
        )
    return (
        response.json()
        if response.content and "json" in response.headers.get("content-type", "")
        else None
    )


def main():
    with (
        httpx.Client(base_url=IDENTITY, timeout=20) as identity,
        httpx.Client(base_url=EVENTS, timeout=30) as api,
        httpx.Client(base_url=MAILPIT, timeout=10) as mailbox,
    ):
        attendee, _ = account(identity, mailbox, "aigerim@example.kz", "Aigerim Zharkeshova", "kk")
        second, _ = account(identity, mailbox, "arman@example.kz", "Arman Nurlanov", "ru")
        staff, _ = account(identity, mailbox, "staff@biletflow.kz", "Door Staff", "ru")
        organizer, user = account(
            identity, mailbox, "organizer@biletflow.kz", "Steppe Live Events", "en"
        )
        if user["role"] != "organizer":
            ok(
                identity.post(
                    "/api/organizer-profile",
                    headers=organizer,
                    json={"contactEmail": "organizer@biletflow.kz"},
                ),
                201,
            )
            organizer, _ = account(
                identity, mailbox, "organizer@biletflow.kz", "Steppe Live Events", "en"
            )
        admin, admin_user = account(
            identity, mailbox, "admin@biletflow.kz", "BiletFlow Admin", "en"
        )
        # A first request registers the admin with the Go service's user directory.
        ok(api.get("/api/notifications", headers=admin))
        if admin_user["role"] != "platform_admin":
            print(
                "Next: make admin@biletflow.kz a platform admin with\n"
                "  docker compose exec python python -m app.cli set-role "
                "admin@biletflow.kz --role platform_admin"
            )

        mine = ok(api.get("/api/events?mine=true", headers=organizer))
        if any(e["title"] == "Astana Jazz Night 2026" for e in mine):
            print("Demo data already loaded.")
            return

        # ── A paid concert with activation, two ticket types, a campaign and sales ──
        jazz = ok(
            api.post(
                "/api/events",
                headers=organizer,
                json={
                    "title": "Astana Jazz Night 2026",
                    "category": "Music",
                    "startsAt": at(12, 19),
                    "endsAt": at(12, 22),
                    "venueName": "Astana Concert Hall",
                    "venueAddress": "Mangilik El Ave 10",
                    "city": "Astana",
                    "capacity": 300,
                    "description": "An evening of live jazz with Kazakh and international "
                    "musicians. Doors open 30 minutes before the start.",
                    "refundPolicy": "Full refund up to 7 days before the event; no refunds "
                    "after that unless the event is cancelled.",
                },
            ),
            201,
        )
        standard = ok(
            api.post(
                f"/api/events/{jazz['id']}/ticket-types",
                headers=organizer,
                json={"name": "Standard", "priceKzt": 8000, "quantity": 220, "maxPerOrder": 6},
            ),
            201,
        )
        vip = ok(
            api.post(
                f"/api/events/{jazz['id']}/ticket-types",
                headers=organizer,
                json={
                    "name": "VIP",
                    "priceKzt": 20000,
                    "quantity": 80,
                    "maxPerOrder": 4,
                    "description": "Front rows and a welcome drink",
                },
            ),
            201,
        )
        ok(api.post(f"/api/events/{jazz['id']}/publish", headers=organizer))
        ok(
            api.put(
                "/api/organizer/payment-setup/identity",
                headers=organizer,
                json={
                    "legalName": "Steppe Live Events LLP",
                    "iin": "990101300123",
                    "consent": True,
                },
            )
        )
        ok(
            api.put(
                "/api/organizer/payment-setup/payout",
                headers=organizer,
                json={
                    "bankName": "Halyk Bank",
                    "accountHolder": "Steppe Live Events LLP",
                    "iban": "KZ86125KZT5004100100",
                },
            )
        )
        ok(api.post(f"/api/events/{jazz['id']}/activation/terms", headers=organizer))
        ok(
            api.post(
                f"/api/events/{jazz['id']}/activation/fee",
                headers=organizer,
                json={"cardNumber": CARD},
            )
        )
        ok(api.post(f"/api/events/{jazz['id']}/activation/activate", headers=organizer))
        ok(
            api.post(
                f"/api/events/{jazz['id']}/campaigns",
                headers=organizer,
                json={
                    "name": "Students",
                    "code": "NU25",
                    "discountType": "percent",
                    "discountValue": 25,
                    "maxRedemptions": 50,
                },
            ),
            201,
        )
        ok(
            api.post(
                f"/api/events/{jazz['id']}/staff",
                headers=organizer,
                json={"email": "staff@biletflow.kz", "canUndo": True},
            ),
            201,
        )

        def buy(headers, name, email, event_id, items, promo=None):
            order = ok(
                api.post(
                    "/api/checkout/reserve",
                    headers=headers,
                    json={
                        "eventId": event_id,
                        "items": [{"ticketTypeId": t, "quantity": q} for t, q in items],
                        "buyer": {"name": name, "email": email},
                        "promoCode": promo,
                    },
                ),
                201,
            )
            if order["status"] == "pending":
                order = ok(
                    api.post(
                        f"/api/checkout/{order['id']}/pay",
                        headers=headers,
                        json={"cardNumber": CARD},
                    )
                )
            return order

        buy(
            attendee, "Aigerim Zharkeshova", "aigerim@example.kz", jazz["id"], [(standard["id"], 2)]
        )
        buy(
            second, "Arman Nurlanov", "arman@example.kz", jazz["id"], [(vip["id"], 1)], promo="NU25"
        )

        # ── A free student meetup ──
        hack = ok(
            api.post(
                "/api/events",
                headers=organizer,
                json={
                    "title": "NU Hackathon Kickoff",
                    "category": "Community",
                    "startsAt": at(5, 18),
                    "endsAt": at(5, 20),
                    "venueName": "Nazarbayev University, Block C3",
                    "venueAddress": "Kabanbay Batyr Ave 53",
                    "city": "Astana",
                    "capacity": 120,
                    "description": "Free kickoff meetup for the spring hackathon: "
                    "team formation, mentors and pizza.",
                },
            ),
            201,
        )
        free = ok(
            api.post(
                f"/api/events/{hack['id']}/ticket-types",
                headers=organizer,
                json={
                    "name": "Free registration",
                    "priceKzt": 0,
                    "quantity": 120,
                    "maxPerOrder": 2,
                },
            ),
            201,
        )
        ok(api.post(f"/api/events/{hack['id']}/publish", headers=organizer))
        ok(
            api.post(
                f"/api/events/{hack['id']}/staff",
                headers=organizer,
                json={"email": "staff@biletflow.kz"},
            ),
            201,
        )
        buy(attendee, "Aigerim Zharkeshova", "aigerim@example.kz", hack["id"], [(free["id"], 1)])

        # ── A workshop in Almaty and a draft ──
        workshop = ok(
            api.post(
                "/api/events",
                headers=organizer,
                json={
                    "title": "Product Design Workshop",
                    "category": "Workshops",
                    "startsAt": at(20, 15),
                    "endsAt": at(20, 18),
                    "venueName": "Almaty Hub",
                    "city": "Almaty",
                    "capacity": 40,
                    "description": "Hands-on Figma workshop for beginners.",
                },
            ),
            201,
        )
        ok(
            api.post(
                f"/api/events/{workshop['id']}/ticket-types",
                headers=organizer,
                json={"name": "Seat", "priceKzt": 0, "quantity": 40},
            ),
            201,
        )
        ok(api.post(f"/api/events/{workshop['id']}/publish", headers=organizer))
        ok(
            api.post(
                "/api/events",
                headers=organizer,
                json={"title": "Winter Design Meetup", "category": "Arts", "startsAt": at(45, 18)},
            ),
            201,
        )

        # ── Support ──
        orders = ok(api.get("/api/orders", headers=attendee))
        jazz_order = next(o for o in orders if o["eventId"] == jazz["id"])
        ok(
            api.post(
                "/api/support/cases",
                headers=attendee,
                json={
                    "category": "event_information",
                    "subject": "Is there parking at the venue?",
                    "message": "Hello! Is there parking near the concert hall?",
                    "orderId": jazz_order["id"],
                },
            ),
            201,
        )
        print(
            "Demo data loaded: 3 published events, 1 draft, orders, a promo campaign, "
            "door staff and a support case."
        )


if __name__ == "__main__":
    main()
