"""Bounded HTTP boundary to Go authority, never direct writes to business data."""

from uuid import UUID

import httpx
from fastapi import HTTPException


class BusinessClient:
    def __init__(self, base_url: str):
        self.base_url = base_url.rstrip("/")
        parsed = httpx.URL(self.base_url)
        if (
            parsed.scheme not in {"http", "https"}
            or not parsed.host
            or parsed.userinfo
            or parsed.query
            or parsed.fragment
        ):
            raise ValueError("BUSINESS_URL must be a trusted HTTP(S) service base")
        self.client = httpx.Client(base_url=self.base_url, timeout=2, follow_redirects=False)

    def close(self):
        self.client.close()

    def get(self, path: str, token: str, request_id: str, params=None):
        return self.request("GET", path, token, request_id, params=params)

    def request(self, method, path, token, request_id, *, params=None, json=None):
        try:
            with self.client.stream(
                method,
                path,
                headers={"Authorization": token, "X-Request-ID": request_id},
                params=params,
                json=json,
            ) as response:
                if response.status_code in {401, 403}:
                    raise HTTPException(response.status_code, "Business context access denied")
                if response.status_code == 404:
                    raise HTTPException(404, "Business context not found")
                if response.status_code < 200 or response.status_code >= 300:
                    raise HTTPException(503, "Business service unavailable")
                body = bytearray()
                for chunk in response.iter_bytes():
                    body.extend(chunk)
                    if len(body) > 262144:
                        raise HTTPException(503, "Business response exceeds limit")
                import json as json_module

                return json_module.loads(body) if body else {}
        except (httpx.HTTPError, ValueError):
            raise HTTPException(503, "Business service unavailable") from None

    def context(
        self, *, actor_id, role, token, request_id, event_id=None, order_id=None, ticket_id=None
    ):
        # Every order/ticket is read through Go with this caller's bearer. A
        # returned organizerId permits private-event support without making the
        # private Event page public to all attendees.
        context_owner = None
        for kind, identity in [("orders", order_id), ("tickets", ticket_id)]:
            if not identity:
                continue
            resource = self.get(f"/api/{kind}/{identity}", token, request_id)
            try:
                if UUID(resource["id"]) != identity:
                    raise ValueError()
                linked = UUID(resource["eventId"])
                supplied_owner = (
                    UUID(resource["organizerId"]) if resource.get("organizerId") else None
                )
                if context_owner and supplied_owner and context_owner != supplied_owner:
                    raise HTTPException(422, "Inconsistent organizer context")
                context_owner = supplied_owner or context_owner
                if kind == "tickets" and order_id and UUID(resource["orderId"]) != order_id:
                    raise HTTPException(422, "Ticket does not belong to the supplied order")
            except (ValueError, KeyError, TypeError):
                raise HTTPException(503, "Invalid business context") from None
            if event_id and linked != event_id:
                raise HTTPException(422, "Inconsistent support context")
            event_id = linked
            if role == "attendee" and str(
                resource.get("userId") or resource.get("attendeeId")
            ) != str(actor_id):
                raise HTTPException(403, "Context belongs to another user")
        if event_id and context_owner:
            return event_id, context_owner
        if not event_id:
            return None, None
        try:
            event = self.get(f"/api/events/{event_id}", token, request_id)
        except HTTPException as original:
            # Main's historical in-memory Go scaffold emits 32-hex IDs. Read-only
            # compatibility while Madi's canonical UUID persistence PR is reviewed.
            if original.status_code != 404:
                raise
            try:
                event = self.get(f"/api/events/{event_id.hex}", token, request_id)
            except HTTPException as legacy:
                if legacy.status_code in {400, 404}:
                    raise original from None
                raise
        try:
            if UUID(event["id"]) != event_id:
                raise ValueError()
            organizer = UUID(event["organizerId"])
        except (KeyError, ValueError, TypeError):
            raise HTTPException(503, "Invalid business context") from None
        return event_id, organizer

    def staff(self, actor, event_id, token, request_id):
        if actor.role == "event_admin":
            assignment = self.get(f"/api/events/{event_id}/staff/me", token, request_id)
            if (
                assignment.get("userId") == str(actor.id)
                and assignment.get("eventId") == str(event_id)
                and assignment.get("canSupport") is True
            ):
                return
            raise HTTPException(403, "Assigned event support permission required")
        _, owner = self.context(
            actor_id=actor.id,
            role=actor.role,
            token=token,
            request_id=request_id,
            event_id=event_id,
        )
        if actor.role == "organizer" and owner == actor.id:
            return
        raise HTTPException(403, "Event support permission required")
