from datetime import UTC, datetime
from uuid import uuid4

from fastapi import HTTPException
from sqlalchemy import or_, select

from app.models.identity import User
from app.models.platform import SupportAction, SupportCase, SupportMessage
from app.services.notifications import digest


class SupportService:
    def __init__(self, session, actor, token, request_id, business, notifications):
        self.session, self.actor, self.token, self.request_id = session, actor, token, request_id
        self.business, self.notifications = business, notifications

    def staff(self, case):
        if self.actor.role == "platform_admin":
            return
        if case.channel == "platform" or not case.event_id:
            raise HTTPException(403, "Support staff permission required")
        self.business.staff(self.actor, case.event_id, self.token, self.request_id)

    def case(self, identity, *, write=False, staff=False):
        query = select(SupportCase).where(SupportCase.id == identity)
        if write:
            query = query.with_for_update().execution_options(populate_existing=True)
        case = self.session.scalar(query)
        if case is None:
            raise HTTPException(404, "Support case not found")
        if staff or case.requester_id != self.actor.id:
            try:
                self.staff(case)
            except HTTPException as exc:
                if exc.status_code in {401, 403, 404} and not staff:
                    raise HTTPException(404, "Support case not found") from None
                raise
        return case

    def create(self, data, key):
        # Account locks serialize retries before the unique request constraint.
        self.session.refresh(self.actor, with_for_update={"key_share": True})
        old = self.session.scalar(
            select(SupportCase).where(
                SupportCase.requester_id == self.actor.id, SupportCase.request_key == key
            )
        )
        hashed = digest(data.model_dump(mode="json", by_alias=True))
        if old:
            if old.request_hash != hashed:
                raise HTTPException(409, "Idempotency key payload mismatch")
            return old, False
        if self.actor.role not in {"attendee", "organizer"}:
            raise HTTPException(403, "Requester role required")
        event_id, owner = self.business.context(
            actor_id=self.actor.id,
            role=self.actor.role,
            token=self.token,
            request_id=self.request_id,
            event_id=data.event_id,
            order_id=data.order_id,
            ticket_id=data.ticket_id,
        )
        if owner and self.session.get(User, owner) is None:
            raise HTTPException(503, "Business organizer identity is unavailable")
        if self.actor.role == "attendee" and not event_id:
            raise HTTPException(422, "Attendee case requires authorized event/order/ticket context")
        if self.actor.role == "organizer" and event_id and owner != self.actor.id:
            raise HTTPException(403, "Own event context required")
        case = SupportCase(
            id=uuid4(),
            requester_id=self.actor.id,
            organizer_id=owner,
            event_id=event_id,
            order_id=data.order_id,
            ticket_id=data.ticket_id,
            channel="event" if self.actor.role == "attendee" else "platform",
            category=data.category,
            subject=data.subject,
            request_key=key,
            request_hash=hashed,
            status="open",
            sequence=1,
        )
        self.session.add(case)
        self.session.flush()
        self.session.add(
            SupportMessage(
                case_id=case.id,
                author_id=self.actor.id,
                body=data.message,
                sequence=1,
                request_key=key,
            )
        )
        action = self.audit(case, key, hashed, "created", "Support case opened")
        self.notify(case, action.id, "support_status")
        self.session.commit()
        return case, True

    def audit(self, case, key, hashed, action, detail):
        row = SupportAction(
            id=uuid4(),
            case_id=case.id,
            actor_id=self.actor.id,
            request_key=key,
            request_hash=hashed,
            action=action,
            detail=detail,
        )
        self.session.add(row)
        return row

    def notify(self, case, source_id, template):
        recipients = {case.requester_id}
        if case.assignee_id:
            recipients.add(case.assignee_id)
        if case.channel == "event" and case.organizer_id:
            recipients.add(case.organizer_id)
        if case.channel == "platform":
            recipients.update(
                self.session.scalars(
                    select(User.id).where(User.role == "platform_admin", User.is_active.is_(True))
                )
            )
        for uid in recipients - {self.actor.id}:
            self.notifications.enqueue(
                self.session, uid, template, "support", source_id, {"caseId": str(case.id)}
            )

    def reply(self, identity, body, key):
        case = self.case(identity, write=True)
        prior = self.session.scalar(
            select(SupportAction).where(
                SupportAction.case_id == identity,
                SupportAction.actor_id == self.actor.id,
                SupportAction.request_key == key,
            )
        )
        if prior and prior.action != "reply":
            raise HTTPException(409, "Idempotency key belongs to a different action")
        old = self.session.scalar(
            select(SupportMessage).where(
                SupportMessage.case_id == identity,
                SupportMessage.author_id == self.actor.id,
                SupportMessage.request_key == key,
            )
        )
        if old:
            if old.body != body:
                raise HTTPException(409, "Idempotency key payload mismatch")
            return old, False
        case.sequence += 1
        case.updated_at = datetime.now(UTC)
        if self.actor.id == case.requester_id and case.status in {
            "resolved",
            "waiting_for_customer",
        }:
            case.status = "open" if case.status == "resolved" else "in_progress"
        message = SupportMessage(
            id=uuid4(),
            case_id=case.id,
            author_id=self.actor.id,
            body=body,
            sequence=case.sequence,
            request_key=key,
        )
        self.session.add(message)
        action = self.audit(case, key, digest({"body": body}), "reply", "Support reply added")
        self.notify(case, action.id, "support_reply")
        self.session.commit()
        return message, True

    def change(self, identity, data, key):
        case = self.case(identity, write=True, staff=True)
        if ("status" in data.model_fields_set and data.status is None) or (
            "escalated" in data.model_fields_set and data.escalated is not True
        ):
            raise HTTPException(422, "Status cannot be null and escalation must be true")
        if not data.model_fields_set:
            raise HTTPException(422, "At least one change is required")
        hashed = digest(data.model_dump(mode="json", by_alias=True, exclude_unset=True))
        old = self.session.scalar(
            select(SupportAction).where(
                SupportAction.case_id == identity,
                SupportAction.actor_id == self.actor.id,
                SupportAction.request_key == key,
            )
        )
        if old:
            if old.request_hash != hashed:
                raise HTTPException(409, "Idempotency key payload mismatch")
            return case
        if "assignee_id" in data.model_fields_set:
            assignee = self.session.get(User, data.assignee_id) if data.assignee_id else None
            if data.assignee_id and (
                not assignee or not assignee.is_active or not assignee.email_verified
            ):
                raise HTTPException(422, "Assignee must be an active verified user")
            if assignee and case.channel == "platform" and assignee.role != "platform_admin":
                raise HTTPException(403, "Platform case requires Platform Admin assignee")
            if assignee and case.channel == "event":
                # Authorization for another user cannot reuse the caller's bearer.
                if assignee.id != self.actor.id and assignee.id != case.organizer_id:
                    raise HTTPException(
                        422, "Event staff must self-assign after live context authorization"
                    )
                if assignee.id == case.organizer_id and assignee.role != "organizer":
                    raise HTTPException(403, "Organizer role required")
            case.assignee_id = data.assignee_id
        if data.status is not None:
            case.status = data.status
        if data.escalated:
            if self.actor.role != "platform_admin":
                self.staff(case)
            case.channel = "platform"
            case.assignee_id = None
        case.updated_at = datetime.now(UTC)
        action = self.audit(case, key, hashed, "changed", "Support status or assignment updated")
        self.notify(
            case,
            action.id,
            "support_assignment" if "assignee_id" in data.model_fields_set else "support_status",
        )
        self.session.commit()
        return case

    def listing(self, *, event_id=None, after=None, limit=50):
        query = select(SupportCase)
        if self.actor.role != "platform_admin":
            if event_id and self.actor.role in {"organizer", "event_admin"}:
                self.business.staff(self.actor, event_id, self.token, self.request_id)
                query = query.where(
                    or_(
                        SupportCase.requester_id == self.actor.id,
                        (SupportCase.event_id == event_id) & (SupportCase.channel == "event"),
                    )
                )
            else:
                query = query.where(
                    or_(
                        SupportCase.requester_id == self.actor.id,
                        SupportCase.assignee_id == self.actor.id,
                        SupportCase.organizer_id == self.actor.id,
                    )
                )
        if event_id:
            query = query.where(SupportCase.event_id == event_id)
        if after:
            query = query.where(SupportCase.id > after)
        rows = list(self.session.scalars(query.order_by(SupportCase.id).limit(limit + 1)))
        visible = []
        for row in rows[:limit]:
            try:
                self.case(row.id)
                visible.append(row)
            except HTTPException as exc:
                if exc.status_code != 404:
                    raise
        return visible, rows[limit - 1].id if len(rows) > limit else None
