from uuid import UUID

import jwt
from fastapi import Depends, HTTPException, Request
from fastapi.security import HTTPAuthorizationCredentials, HTTPBearer
from sqlalchemy.orm import Session

from app.db.session import get_session
from app.models.identity import Role, User

bearer = HTTPBearer(auto_error=False)


def unauthorized():
    return HTTPException(
        401, "Invalid or missing access token", headers={"WWW-Authenticate": "Bearer"}
    )


def current_user(
    request: Request,
    credentials: HTTPAuthorizationCredentials | None = Depends(bearer),
    session: Session = Depends(get_session),
) -> User:
    if credentials is None or len(credentials.credentials) > 8192:
        raise unauthorized()
    try:
        claims = request.app.state.tokens.decode(credentials.credentials)
        uid = UUID(claims["sub"])
        Role(claims["role"])
    except (jwt.InvalidTokenError, ValueError, TypeError, KeyError):
        raise unauthorized() from None
    user = session.get(User, uid)
    if (
        not user
        or not user.is_active
        or not user.email_verified
        or user.token_version != claims["ver"]
    ):
        raise unauthorized()
    return user


def require_roles(*roles: Role):
    def dependency(user: User = Depends(current_user)) -> User:
        if Role(user.role) not in roles:
            raise HTTPException(403, "Insufficient permissions")
        return user

    return dependency
