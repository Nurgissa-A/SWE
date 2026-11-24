"""Transactional encrypted jobs; SMTP side effects never repeat business writes."""

import hashlib
import json
from datetime import UTC, datetime, timedelta
from uuid import UUID, uuid4

from cryptography.fernet import Fernet, InvalidToken
from fastapi import HTTPException
from sqlalchemy import and_, or_, select
from sqlalchemy.dialects.postgresql import insert

from app.models.identity import ActionToken, EmailDelivery, User
from app.models.platform import Notification

TITLES = {
    "verify_email": "Verify your BiletFlow email",
    "reset_password": "Reset your BiletFlow password",
    "order_confirmation": "Registration or simulated purchase confirmed",
    "ticket_delivery": "Your admission ticket is available",
    "payment_failed": "Simulated payment failed",
    "event_updated": "Your event was updated",
    "event_cancelled": "Your event was cancelled",
    "refund_completed": "Simulated refund completed",
    "payout_status": "Simulated payout status updated",
    "support_reply": "New support reply",
    "support_assignment": "Support assignment changed",
    "support_status": "Support status changed",
    "account_status": "Account access updated",
}
FINANCIAL = {"order_confirmation", "payment_failed", "refund_completed", "payout_status"}


def digest(payload: dict) -> str:
    return hashlib.sha256(
        json.dumps(payload, sort_keys=True, separators=(",", ":")).encode()
    ).hexdigest()


class Notifications:
    def __init__(self, key: bytes, frontend_url: str):
        self.cipher = Fernet(key)
        self.frontend_url = frontend_url.rstrip("/")

    def decode(self, row: Notification) -> dict:
        return json.loads(self.cipher.decrypt(row.encrypted_payload.encode()))

    def enqueue(
        self,
        session,
        user_id: UUID,
        template: str,
        source: str,
        source_id: UUID,
        payload: dict,
        action_token_id: UUID | None = None,
    ) -> Notification:
        if template not in TITLES:
            raise ValueError("Unsupported notification template")
        hashed = digest(payload)
        statement = (
            insert(Notification)
            .values(
                source=source,
                source_id=source_id,
                user_id=user_id,
                template=template,
                encrypted_payload=self.cipher.encrypt(json.dumps(payload).encode()).decode(),
                payload_hash=hashed,
                action_token_id=action_token_id,
                state="pending",
                attempts=0,
                next_attempt_at=datetime.now(UTC),
                created_at=datetime.now(UTC),
            )
            .on_conflict_do_nothing(constraint="uq_notification_action")
            .returning(Notification.id)
        )
        session.scalar(statement)
        row = session.scalar(
            select(Notification).where(
                Notification.source == source,
                Notification.source_id == source_id,
                Notification.user_id == user_id,
                Notification.template == template,
            )
        )
        if row.payload_hash != hashed:
            raise HTTPException(
                409, "Idempotency key already used with different notification data"
            )
        return row

    def public(self, row: Notification) -> dict:
        payload = self.decode(row)
        return {
            "id": row.id,
            "template": row.template,
            "title": TITLES[row.template],
            "context": {key: value for key, value in payload.items() if key.endswith("Id")},
            "link": self.link(payload),
            "state": row.state,
            "attempts": row.attempts,
            "readAt": row.read_at,
            "createdAt": row.created_at,
        }

    def link(self, payload: dict) -> str:
        for key, route in [
            ("caseId", "support"),
            ("ticketId", "tickets"),
            ("orderId", "orders"),
            ("eventId", "events"),
        ]:
            if payload.get(key):
                return self.frontend_url + "/" + route + "/" + str(UUID(str(payload[key])))
        return self.frontend_url + "/dashboard"

    def drain(self, factory, mailer, *, limit: int = 50, only_id: int | None = None) -> int:
        delivered = 0
        for _ in range(limit):
            now = datetime.now(UTC)
            with factory() as session:
                query = (
                    select(Notification)
                    .where(
                        or_(
                            and_(
                                Notification.state == "pending", Notification.next_attempt_at <= now
                            ),
                            and_(Notification.state == "sending", Notification.lease_until <= now),
                        )
                    )
                    .order_by(Notification.id)
                    .with_for_update(skip_locked=True)
                    .limit(1)
                )
                if only_id is not None:
                    query = query.where(Notification.id == only_id)
                row = session.scalar(query)
                if row is None:
                    break
                user = session.execute(
                    select(User.email, User.email_verified, User.is_active).where(
                        User.id == row.user_id
                    )
                ).one_or_none()
                token = (
                    session.execute(
                        select(
                            ActionToken.user_id, ActionToken.used_at, ActionToken.expires_at
                        ).where(ActionToken.id == row.action_token_id)
                    ).one_or_none()
                    if row.action_token_id
                    else None
                )
                stale = row.action_token_id and (
                    not token
                    or token.user_id != row.user_id
                    or token.used_at
                    or token.expires_at <= now
                )
                if (
                    not user
                    or not user.is_active
                    or stale
                    or (row.source != "identity" and not user.email_verified)
                    or (row.template == "verify_email" and user.email_verified)
                ):
                    row.state, row.failure_code = "skipped", "inactive_or_expired"
                    session.commit()
                    continue
                if row.attempts >= 5:
                    row.state, row.failure_code = "failed", "retry_limit"
                    session.commit()
                    continue
                row.state, row.lease_id = "sending", uuid4()
                row.lease_until = now + timedelta(seconds=60)
                row.attempts += 1
                job_id, lease_id, recipient, template = (
                    row.id,
                    row.lease_id,
                    user.email,
                    row.template,
                )
                try:
                    payload = self.decode(row)
                except (InvalidToken, ValueError, KeyError):
                    row.state, row.failure_code = "failed", "payload_unreadable"
                    session.commit()
                    continue
                session.commit()
            failure = None
            try:
                message_id = f"<notification-{job_id}@biletflow>"
                if template in {"verify_email", "reset_password"}:
                    mailer.send(recipient, template, payload["token"], message_id=message_id)
                else:
                    note = (
                        "Simulation only. No real money moved.\n" if template in FINANCIAL else ""
                    )
                    mailer.send_notification(
                        recipient,
                        TITLES[template],
                        note + self.link(payload),
                        message_id=message_id,
                    )
            except Exception:
                # SMTP/provider errors may contain addresses or server credentials.
                failure = "delivery_unavailable"
            with factory() as session:
                row = session.get(Notification, job_id, with_for_update=True)
                if row is None or row.lease_id != lease_id or row.state != "sending":
                    continue
                row.lease_until, row.lease_id, row.failure_code = None, None, failure
                if failure:
                    row.state = "failed" if row.attempts >= 5 else "pending"
                    row.next_attempt_at = datetime.now(UTC) + timedelta(
                        seconds=min(3600, 30 * 2 ** (row.attempts - 1))
                    )
                else:
                    row.state, row.delivered_at = "delivered", datetime.now(UTC)
                    delivered += 1
                if row.action_token_id:
                    receipt = session.scalar(
                        select(EmailDelivery).where(
                            EmailDelivery.action_token_id == row.action_token_id
                        )
                    )
                    if receipt is None:
                        receipt = EmailDelivery(action_token_id=row.action_token_id)
                        session.add(receipt)
                    receipt.attempts, receipt.sent_at = row.attempts, row.delivered_at
                session.commit()
        return delivered
