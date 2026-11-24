from datetime import UTC, datetime
from uuid import UUID

from fastapi import APIRouter, Depends, HTTPException, Request, Response
from sqlalchemy.exc import IntegrityError
from sqlalchemy.orm import Session

from app.api.dependencies import current_user, current_user_for_update, require_roles
from app.core.abuse import auth_budget
from app.db.session import get_session
from app.models.identity import OrganizerProfile, Role, User
from app.models.platform import RevokedToken
from app.repositories.identity import IdentityRepository
from app.schemas.identity import (
    Credentials,
    Login,
    LoginResponse,
    ProfileInput,
    ProfileResponse,
    ResetPassword,
    ResetRequest,
    UserResponse,
    UserUpdate,
    VerifyToken,
)
from app.services.identity import IdentityService

router = APIRouter(prefix="/api", tags=["identity"])


def service(request: Request, session: Session = Depends(get_session)) -> IdentityService:
    return IdentityService(session, request.app.state.settings, request.app.state.mailer)


@router.post(
    "/auth/register",
    response_model=UserResponse,
    status_code=201,
    dependencies=[Depends(auth_budget)],
)
def register(data: Credentials, svc: IdentityService = Depends(service)):
    return svc.register(str(data.email), data.password)


@router.post("/auth/verify-email", status_code=200, response_class=Response)
def verify_email(data: VerifyToken, svc: IdentityService = Depends(service)):
    svc.verify(data.token)


@router.post("/auth/login", response_model=LoginResponse, dependencies=[Depends(auth_budget)])
def login(
    data: Login, request: Request, response: Response, svc: IdentityService = Depends(service)
):
    user = svc.login(str(data.email), data.password)
    response.headers["Cache-Control"] = "no-store"
    return {"token": request.app.state.tokens.issue(user), "user": user}


@router.post(
    "/auth/request-password-reset",
    status_code=200,
    response_class=Response,
    dependencies=[Depends(auth_budget)],
)
def request_reset(data: ResetRequest, svc: IdentityService = Depends(service)):
    svc.request_reset(str(data.email))


@router.post("/auth/reset-password", status_code=200, response_class=Response)
def reset(data: ResetPassword, svc: IdentityService = Depends(service)):
    svc.reset(data.token, data.new_password)


@router.get("/users/me", response_model=UserResponse)
def me(user: User = Depends(current_user)):
    return user


@router.patch("/users/me", response_model=UserResponse)
def update_me(
    data: UserUpdate,
    user: User = Depends(current_user_for_update),
    session: Session = Depends(get_session),
):
    user.display_name = data.display_name
    session.commit()
    return user


@router.post("/organizer-profile", response_model=ProfileResponse, status_code=201)
def create_profile(
    data: ProfileInput,
    user: User = Depends(require_roles(Role.ATTENDEE, Role.ORGANIZER, for_update=True)),
    session: Session = Depends(get_session),
):
    if IdentityRepository(session).profile(user.id):
        raise HTTPException(409, "Organizer profile already exists")
    profile = OrganizerProfile(user_id=user.id, **data.model_dump())
    session.add(profile)
    user.role = Role.ORGANIZER.value
    try:
        session.commit()
    except IntegrityError:
        session.rollback()
        raise HTTPException(409, "Organizer profile already exists") from None
    return profile


@router.get("/organizer-profile", response_model=ProfileResponse)
def profile(
    user: User = Depends(require_roles(Role.ORGANIZER)), session: Session = Depends(get_session)
):
    row = IdentityRepository(session).profile(user.id)
    if row is None:
        raise HTTPException(404, "Organizer profile not found")
    return row


@router.patch("/organizer-profile", response_model=ProfileResponse)
def update_profile(
    data: ProfileInput,
    user: User = Depends(require_roles(Role.ORGANIZER, for_update=True)),
    session: Session = Depends(get_session),
):
    row = IdentityRepository(session).profile(user.id)
    if row is None:
        raise HTTPException(404, "Organizer profile not found")
    for key, value in data.model_dump().items():
        setattr(row, key, value)
    session.commit()
    return row


@router.post(
    "/auth/resend-verification",
    status_code=200,
    response_class=Response,
    dependencies=[Depends(auth_budget)],
)
def resend_verification(data: ResetRequest, svc: IdentityService = Depends(service)):
    user = svc.repo.by_email(str(data.email), for_update=True)
    if user and user.is_active and not user.email_verified:
        job = svc.send_action(user, "verify_email", svc.settings.verification_token_minutes)
        svc.session.commit()
        if svc.settings.notification_eager_delivery:
            svc.deliver(job.id)


@router.post("/auth/logout", status_code=204, response_class=Response)
def logout(
    request: Request,
    user: User = Depends(current_user_for_update),
    session: Session = Depends(get_session),
):
    claims = request.app.state.tokens.decode(request.headers["Authorization"].split()[1])
    try:
        expiry = datetime.fromtimestamp(claims["exp"], UTC)
    except (ValueError, OverflowError):
        raise HTTPException(401, "Invalid token expiry") from None
    session.add(RevokedToken(jti=UUID(claims["jti"]), user_id=user.id, expires_at=expiry))
    session.commit()
