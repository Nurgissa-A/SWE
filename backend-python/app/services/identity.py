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
from app.models.identity import ActionToken, Role, User
from app.repositories.identity import IdentityRepository
from app.services.email import EmailSender
from app.services.notifications import Notifications


class IdentityService:
    def __init__(self, session: Session, settings: Settings, mailer: EmailSender):
        self.session, self.settings, self.mailer = session, settings, mailer
        self.repo = IdentityRepository(session)
        self.notifications = Notifications(
            settings.notification_key_file.read_bytes(), settings.frontend_url
        )

    def send_action(self, user: User, purpose: str, minutes: int):
        raw, digest = opaque_token()
        token = ActionToken(
            user_id=user.id,
            token_hash=digest,
            purpose=purpose,
            expires_at=datetime.now(UTC) + timedelta(minutes=minutes),
        )
        self.session.add(token)
        self.session.flush()
        return self.notifications.enqueue(
            self.session, user.id, purpose, "identity", token.id, {"token": raw}, token.id
        )

    def deliver(self, job_id: int) -> None:
        from sqlalchemy.orm import sessionmaker

        self.notifications.drain(
            sessionmaker(bind=self.session.get_bind(), expire_on_commit=False),
            self.mailer,
            limit=1,
            only_id=job_id,
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
        job = self.send_action(user, "verify_email", self.settings.verification_token_minutes)
        self.session.commit()
        if self.settings.notification_eager_delivery:
            self.deliver(job.id)
        return user

    def consume(self, raw: str, purpose: str) -> User:
        # All account actions lock user first, then action tokens. Locking a
        # different reset token first can deadlock when reset invalidates siblings.
        uid = self.repo.token_owner(raw, purpose)
        if uid is None:
            raise HTTPException(400, "Invalid or expired token")
        user = self.session.scalar(
            select(User)
            .where(User.id == uid)
            .with_for_update()
            .execution_options(populate_existing=True)
        )
        token = self.repo.lock_token(raw, purpose)
        now = datetime.now(UTC)  # Expiry is evaluated after any lock wait.
        if (
            not user
            or not user.is_active
            or not token
            or token.user_id != uid
            or token.used_at is not None
            or token.expires_at <= now
        ):
            raise HTTPException(400, "Invalid or expired token")
        token.used_at = now
        return user

    def verify(self, raw: str) -> None:
        user = self.consume(raw, "verify_email")
        user.email_verified = True
        self.session.commit()

    def login(self, email: str, password: str) -> User:
        user = self.repo.by_email(email, for_update=True)
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
        user = self.repo.by_email(email, for_update=True)
        if user and user.is_active:
            job = self.send_action(user, "reset_password", self.settings.reset_token_minutes)
            self.session.commit()
            if self.settings.notification_eager_delivery:
                self.deliver(job.id)

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
