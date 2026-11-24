from uuid import UUID

from sqlalchemy import select
from sqlalchemy.orm import Session

from app.core.security import token_digest
from app.models.identity import ActionToken, OrganizerProfile, User


class IdentityRepository:
    def __init__(self, session: Session):
        self.session = session

    def by_email(self, email: str, *, for_update: bool = False) -> User | None:
        query = select(User).where(User.email == email)
        if for_update:
            query = query.with_for_update().execution_options(populate_existing=True)
        return self.session.scalar(query)

    def token_owner(self, raw: str, purpose: str) -> UUID | None:
        return self.session.scalar(
            select(ActionToken.user_id).where(
                ActionToken.token_hash == token_digest(raw), ActionToken.purpose == purpose
            )
        )

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
