"""Python-owned support, delivery and administration; never Go business rows."""

from datetime import datetime
from uuid import UUID, uuid4

from sqlalchemy import (
    BigInteger,
    CheckConstraint,
    DateTime,
    ForeignKey,
    Identity,
    Index,
    String,
    Text,
    UniqueConstraint,
)
from sqlalchemy.orm import Mapped, mapped_column

from app.db.session import Base
from app.models.identity import now


class Notification(Base):
    __tablename__ = "platform_notifications"
    __table_args__ = (
        UniqueConstraint(
            "source", "source_id", "user_id", "template", name="uq_notification_action"
        ),
        CheckConstraint(
            "state IN ('pending','sending','delivered','failed','skipped')",
            name="ck_notification_state",
        ),
        CheckConstraint("attempts >= 0", name="ck_notification_attempts"),
        Index("ix_notification_due", "state", "next_attempt_at"),
        Index("ix_notification_user_cursor", "user_id", "id"),
    )
    id: Mapped[int] = mapped_column(BigInteger, Identity(), primary_key=True)
    source: Mapped[str] = mapped_column(String(24))
    source_id: Mapped[UUID]
    user_id: Mapped[UUID] = mapped_column(ForeignKey("identity_users.id", ondelete="CASCADE"))
    template: Mapped[str] = mapped_column(String(40))
    encrypted_payload: Mapped[str] = mapped_column(Text)
    payload_hash: Mapped[str] = mapped_column(String(64))
    action_token_id: Mapped[UUID | None] = mapped_column(
        ForeignKey("identity_action_tokens.id", ondelete="CASCADE")
    )
    state: Mapped[str] = mapped_column(String(16), default="pending")
    attempts: Mapped[int] = mapped_column(default=0)
    next_attempt_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), default=now)
    lease_until: Mapped[datetime | None] = mapped_column(DateTime(timezone=True))
    lease_id: Mapped[UUID | None]
    delivered_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True))
    read_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True))
    failure_code: Mapped[str | None] = mapped_column(String(32))
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), default=now)


class SupportCase(Base):
    __tablename__ = "platform_support_cases"
    __table_args__ = (
        UniqueConstraint("requester_id", "request_key", name="uq_case_request"),
        CheckConstraint("channel IN ('event','platform')", name="ck_case_channel"),
        CheckConstraint(
            "status IN ('open','in_progress','waiting_for_customer','resolved')",
            name="ck_case_status",
        ),
        CheckConstraint("sequence >= 1", name="ck_case_sequence"),
        Index("ix_case_requester", "requester_id", "id"),
        Index("ix_case_event", "event_id", "updated_at"),
        Index("ix_case_assignee", "assignee_id", "updated_at"),
    )
    id: Mapped[UUID] = mapped_column(primary_key=True, default=uuid4)
    requester_id: Mapped[UUID] = mapped_column(ForeignKey("identity_users.id", ondelete="RESTRICT"))
    organizer_id: Mapped[UUID | None]  # validated immutable reference, not a Go table copy
    assignee_id: Mapped[UUID | None] = mapped_column(
        ForeignKey("identity_users.id", ondelete="RESTRICT")
    )
    event_id: Mapped[UUID | None]
    order_id: Mapped[UUID | None]
    ticket_id: Mapped[UUID | None]
    channel: Mapped[str] = mapped_column(String(16))
    category: Mapped[str] = mapped_column(String(24))
    subject: Mapped[str] = mapped_column(String(160))
    status: Mapped[str] = mapped_column(String(24), default="open")
    request_key: Mapped[UUID]
    request_hash: Mapped[str] = mapped_column(String(64))
    sequence: Mapped[int] = mapped_column(default=1)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), default=now)
    updated_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), default=now)


class SupportMessage(Base):
    __tablename__ = "platform_support_messages"
    __table_args__ = (
        UniqueConstraint("case_id", "sequence", name="uq_message_sequence"),
        UniqueConstraint("case_id", "author_id", "request_key", name="uq_message_request"),
        CheckConstraint("length(btrim(body)) > 0", name="ck_message_body"),
    )
    id: Mapped[UUID] = mapped_column(primary_key=True, default=uuid4)
    case_id: Mapped[UUID] = mapped_column(
        ForeignKey("platform_support_cases.id", ondelete="RESTRICT")
    )
    author_id: Mapped[UUID] = mapped_column(ForeignKey("identity_users.id", ondelete="RESTRICT"))
    sequence: Mapped[int]
    body: Mapped[str] = mapped_column(Text)
    request_key: Mapped[UUID]
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), default=now)


class SupportAction(Base):
    __tablename__ = "platform_support_actions"
    __table_args__ = (
        UniqueConstraint("case_id", "actor_id", "request_key", name="uq_support_action"),
    )
    id: Mapped[UUID] = mapped_column(primary_key=True, default=uuid4)
    case_id: Mapped[UUID] = mapped_column(
        ForeignKey("platform_support_cases.id", ondelete="RESTRICT")
    )
    actor_id: Mapped[UUID] = mapped_column(ForeignKey("identity_users.id", ondelete="RESTRICT"))
    request_key: Mapped[UUID]
    request_hash: Mapped[str] = mapped_column(String(64))
    action: Mapped[str] = mapped_column(String(32))
    detail: Mapped[str] = mapped_column(String(160))
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), default=now)


class PlatformSettings(Base):
    __tablename__ = "platform_settings"
    __table_args__ = (
        CheckConstraint("id = 1", name="ck_platform_singleton"),
        CheckConstraint(
            "activation_fee_minor >= 0 AND activation_fee_minor <= 100000000",
            name="ck_activation_fee",
        ),
    )
    id: Mapped[int] = mapped_column(primary_key=True, default=1)
    activation_fee_minor: Mapped[int] = mapped_column(default=0)
    version: Mapped[int] = mapped_column(default=1)
    updated_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), default=now)


class AdminAudit(Base):
    __tablename__ = "platform_admin_audit"
    __table_args__ = (
        CheckConstraint("actor_kind IN ('account','operator')", name="ck_audit_actor_kind"),
    )
    actor_kind: Mapped[str] = mapped_column(String(16), default="account")
    id: Mapped[int] = mapped_column(BigInteger, Identity(), primary_key=True)
    actor_id: Mapped[UUID | None] = mapped_column(
        ForeignKey("identity_users.id", ondelete="RESTRICT")
    )
    action: Mapped[str] = mapped_column(String(40))
    target_id: Mapped[str] = mapped_column(String(64))
    detail: Mapped[str] = mapped_column(String(160))
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), default=now)


class RateLimit(Base):
    __tablename__ = "identity_rate_limits"
    key: Mapped[str] = mapped_column(String(64), primary_key=True)
    count: Mapped[int] = mapped_column(default=1)
    expires_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), index=True)


class RevokedToken(Base):
    __tablename__ = "identity_revoked_tokens"
    jti: Mapped[UUID] = mapped_column(primary_key=True)
    user_id: Mapped[UUID] = mapped_column(ForeignKey("identity_users.id", ondelete="CASCADE"))
    expires_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), index=True)
    revoked_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), default=now)


class EventImage(Base):
    __tablename__ = "platform_event_images"
    __table_args__ = (
        CheckConstraint(
            "width > 0 AND height > 0 AND width <= 4096 AND height <= 4096 "
            "AND width * height <= 8000000",
            name="ck_image_dimensions",
        ),
    )
    event_id: Mapped[UUID] = mapped_column(primary_key=True)
    created_by: Mapped[UUID] = mapped_column(ForeignKey("identity_users.id", ondelete="RESTRICT"))
    sha256: Mapped[str] = mapped_column(String(64), index=True)
    width: Mapped[int]
    height: Mapped[int]
    updated_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), default=now)
