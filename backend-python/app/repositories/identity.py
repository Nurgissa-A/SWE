from uuid import UUID

from sqlalchemy import select
from sqlalchemy.orm import Session

from app.core.security import token_digest
from app.models.identity import ActionToken, OrganizerProfile, User


class IdentityRepository:
    def __init__(self, session: Session):
        self.session = session

    def by_email(self, email: str) -> User | None:
        return self.session.scalar(select(User).where(User.email == email))

    def user(self, uid: UUID) -> User | None:
        return self.session.get(User, uid)

    def lock_token(self, raw: str, purpose: str) -> ActionToken | None:
        return self.session.scalar(
            select(ActionToken)
            .where(ActionToken.token_hash == token_digest(raw), ActionToken.purpose == purpose)
            .with_for_update()
        )

    def profile(self, uid: UUID) -> OrganizerProfile | None:
        return self.session.scalar(select(OrganizerProfile).where(OrganizerProfile.user_id == uid))
