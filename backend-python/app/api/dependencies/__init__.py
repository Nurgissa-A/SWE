from uuid import UUID

import jwt
from fastapi import Depends, HTTPException, Request
from fastapi.security import HTTPAuthorizationCredentials, HTTPBearer
from sqlalchemy.orm import Session

from app.db.session import get_session
from app.models.identity import Role, User
from app.models.platform import RevokedToken

bearer = HTTPBearer(auto_error=False)


def unauthorized():
    return HTTPException(
        401, "Invalid or missing access token", headers={"WWW-Authenticate": "Bearer"}
    )


def resolve_user(
    request: Request,
    credentials: HTTPAuthorizationCredentials | None,
    session: Session,
    *,
    for_update: bool = False,
) -> User:
    if credentials is None or len(credentials.credentials) > 8192:
        raise unauthorized()
    try:
        claims = request.app.state.tokens.decode(credentials.credentials)
        uid = UUID(claims["sub"])
        Role(claims["role"])
    except (jwt.InvalidTokenError, ValueError, TypeError, KeyError):
        raise unauthorized() from None
    user = session.get(
        User,
        uid,
        with_for_update={"key_share": True} if for_update else None,
        populate_existing=for_update,
    )
    if (
        session.get(RevokedToken, UUID(claims["jti"])) is not None
        or not user
        or not user.is_active
        or not user.email_verified
        or user.token_version != claims["ver"]
    ):
        raise unauthorized()
    return user


def current_user(
    request: Request,
    credentials: HTTPAuthorizationCredentials | None = Depends(bearer),
    session: Session = Depends(get_session),
) -> User:
    return resolve_user(request, credentials, session)


def current_user_for_update(
    request: Request,
    credentials: HTTPAuthorizationCredentials | None = Depends(bearer),
    session: Session = Depends(get_session),
) -> User:
    # Recheck status, token version and role after acquiring the account lock.
    return resolve_user(request, credentials, session, for_update=True)


def require_roles(*roles: Role, for_update: bool = False):
    lookup = current_user_for_update if for_update else current_user

    def dependency(user: User = Depends(lookup)) -> User:
        if Role(user.role) not in roles:
            raise HTTPException(403, "Insufficient permissions")
        return user

    return dependency
