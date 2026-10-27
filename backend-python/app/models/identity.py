import enum
from datetime import UTC, datetime
from uuid import UUID, uuid4

from sqlalchemy import Boolean, CheckConstraint, DateTime, ForeignKey, String, UniqueConstraint
from sqlalchemy.orm import Mapped, mapped_column

from app.db.session import Base


def now() -> datetime:
    return datetime.now(UTC)


class Role(enum.StrEnum):
    ATTENDEE = "attendee"
    ORGANIZER = "organizer"
    EVENT_ADMIN = "event_admin"
    PLATFORM_ADMIN = "platform_admin"


class User(Base):
    __tablename__ = "identity_users"
    __table_args__ = (
        CheckConstraint(
            "role IN ('attendee','organizer','event_admin','platform_admin')", name="ck_user_role"
        ),
        CheckConstraint("token_version >= 0", name="ck_token_version"),
        CheckConstraint("email = lower(email)", name="ck_email_normalized"),
    )
    id: Mapped[UUID] = mapped_column(primary_key=True, default=uuid4)
    email: Mapped[str] = mapped_column(String(254), unique=True)
    password_hash: Mapped[str] = mapped_column(String(512))
    role: Mapped[str] = mapped_column(String(32), default=Role.ATTENDEE.value)
    display_name: Mapped[str | None] = mapped_column(String(100))
    email_verified: Mapped[bool] = mapped_column(Boolean, default=False)
    is_active: Mapped[bool] = mapped_column(Boolean, default=True)
    token_version: Mapped[int] = mapped_column(default=0)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), default=now)


class OrganizerProfile(Base):
    __tablename__ = "identity_organizer_profiles"
    id: Mapped[UUID] = mapped_column(primary_key=True, default=uuid4)
    user_id: Mapped[UUID] = mapped_column(
        ForeignKey("identity_users.id", ondelete="RESTRICT"), unique=True
    )
    contact_email: Mapped[str] = mapped_column(String(254))
    contact_phone: Mapped[str | None] = mapped_column(String(32))
    payout_account_placeholder: Mapped[str | None] = mapped_column(String(100))


class ActionToken(Base):
    __tablename__ = "identity_action_tokens"
    __table_args__ = (
        CheckConstraint("purpose IN ('verify_email','reset_password')", name="ck_token_purpose"),
    )
    id: Mapped[UUID] = mapped_column(primary_key=True, default=uuid4)
    user_id: Mapped[UUID] = mapped_column(
        ForeignKey("identity_users.id", ondelete="CASCADE"), index=True
    )
    token_hash: Mapped[str] = mapped_column(String(64), unique=True)
    purpose: Mapped[str] = mapped_column(String(32))
    expires_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), index=True)
    used_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True))


class EmailDelivery(Base):
    __tablename__ = "identity_email_deliveries"
    __table_args__ = (UniqueConstraint("action_token_id"),)
    id: Mapped[UUID] = mapped_column(primary_key=True, default=uuid4)
    action_token_id: Mapped[UUID] = mapped_column(
        ForeignKey("identity_action_tokens.id", ondelete="CASCADE")
    )
    # No raw verification/reset token or password is persisted in this table.
    sent_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True))
    attempts: Mapped[int] = mapped_column(default=0)
