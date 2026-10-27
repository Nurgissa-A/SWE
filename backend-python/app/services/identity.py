import smtplib
from datetime import UTC, datetime, timedelta

from fastapi import HTTPException
from sqlalchemy import select, update
from sqlalchemy.exc import IntegrityError
from sqlalchemy.orm import Session

from app.core.config import Settings
from app.core.security import (
    DUMMY_HASH,
    hash_password,
    opaque_token,
    password_hasher,
    verify_password,
)
from app.models.identity import ActionToken, EmailDelivery, Role, User
from app.repositories.identity import IdentityRepository
from app.services.email import EmailSender


class IdentityService:
    def __init__(self, session: Session, settings: Settings, mailer: EmailSender):
        self.session, self.settings, self.mailer = session, settings, mailer
        self.repo = IdentityRepository(session)

    def send_action(self, user: User, purpose: str, minutes: int) -> None:
        raw, digest = opaque_token()
        token = ActionToken(
            user_id=user.id,
            token_hash=digest,
            purpose=purpose,
            expires_at=datetime.now(UTC) + timedelta(minutes=minutes),
        )
        self.session.add(token)
        self.session.flush()
        # Delivery failure rolls back registration/reset. No usable raw token is stored in DB/logs.
        try:
            self.mailer.send(user.email, purpose, raw)
        except (OSError, smtplib.SMTPException):
            self.session.rollback()
            raise HTTPException(503, "Email delivery unavailable; retry later") from None
        self.session.add(
            EmailDelivery(action_token_id=token.id, attempts=1, sent_at=datetime.now(UTC))
        )

    def register(self, email: str, password: str) -> User:
        if self.repo.by_email(email):
            raise HTTPException(409, "Email already registered")
        user = User(email=email, password_hash=hash_password(password), role=Role.ATTENDEE.value)
        self.session.add(user)
        try:
            self.session.flush()
        except IntegrityError:
            self.session.rollback()
            raise HTTPException(409, "Email already registered") from None
        self.send_action(user, "verify_email", self.settings.verification_token_minutes)
        self.session.commit()
        return user

    def consume(self, raw: str, purpose: str) -> User:
        token = self.repo.lock_token(raw, purpose)
        now = datetime.now(UTC)
        if not token or token.used_at is not None or token.expires_at <= now:
            raise HTTPException(400, "Invalid or expired token")
        user = self.session.scalar(select(User).where(User.id == token.user_id).with_for_update())
        if not user or not user.is_active:
            raise HTTPException(400, "Invalid or expired token")
        token.used_at = now
        return user

    def verify(self, raw: str) -> None:
        user = self.consume(raw, "verify_email")
        user.email_verified = True
        self.session.commit()

    def login(self, email: str, password: str) -> User:
        user = self.repo.by_email(email)
        valid = verify_password(user.password_hash if user else DUMMY_HASH, password)
        if not user or not valid or not user.is_active:
            raise HTTPException(401, "Invalid credentials", headers={"WWW-Authenticate": "Bearer"})
        if not user.email_verified:
            raise HTTPException(403, "Email verification required")
        if password_hasher.check_needs_rehash(user.password_hash):
            user.password_hash = hash_password(password)
            self.session.commit()
        return user

    def request_reset(self, email: str) -> None:
        user = self.repo.by_email(email)
        if user and user.is_active:
            self.send_action(user, "reset_password", self.settings.reset_token_minutes)
            self.session.commit()

    def reset(self, raw: str, password: str) -> None:
        user = self.consume(raw, "reset_password")
        user.password_hash = hash_password(password)
        user.token_version += 1
        # Invalidate all other outstanding reset links in the same transaction.
        self.session.execute(
            update(ActionToken)
            .where(
                ActionToken.user_id == user.id,
                ActionToken.purpose == "reset_password",
                ActionToken.used_at.is_(None),
            )
            .values(used_at=datetime.now(UTC))
        )
        self.session.commit()
