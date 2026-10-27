from fastapi import APIRouter, Depends, HTTPException, Request, Response
from sqlalchemy.exc import IntegrityError
from sqlalchemy.orm import Session

from app.api.dependencies import current_user, require_roles
from app.db.session import get_session
from app.models.identity import OrganizerProfile, Role, User
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


@router.post("/auth/register", response_model=UserResponse, status_code=201)
def register(data: Credentials, svc: IdentityService = Depends(service)):
    return svc.register(str(data.email), data.password)


@router.post("/auth/verify-email", status_code=200, response_class=Response)
def verify_email(data: VerifyToken, svc: IdentityService = Depends(service)):
    svc.verify(data.token)


@router.post("/auth/login", response_model=LoginResponse)
def login(
    data: Login, request: Request, response: Response, svc: IdentityService = Depends(service)
):
    user = svc.login(str(data.email), data.password)
    response.headers["Cache-Control"] = "no-store"
    return {"token": request.app.state.tokens.issue(user), "user": user}


@router.post("/auth/request-password-reset", status_code=200, response_class=Response)
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
    data: UserUpdate, user: User = Depends(current_user), session: Session = Depends(get_session)
):
    user.display_name = data.display_name
    session.commit()
    return user


@router.post("/organizer-profile", response_model=ProfileResponse, status_code=201)
def create_profile(
    data: ProfileInput,
    user: User = Depends(require_roles(Role.ATTENDEE, Role.ORGANIZER)),
    session: Session = Depends(get_session),
):
    # Lock current user to serialize simultaneous onboarding requests for one account.
    session.refresh(user, with_for_update=True)
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
    user: User = Depends(require_roles(Role.ORGANIZER)),
    session: Session = Depends(get_session),
):
    row = IdentityRepository(session).profile(user.id)
    if row is None:
        raise HTTPException(404, "Organizer profile not found")
    for key, value in data.model_dump().items():
        setattr(row, key, value)
    session.commit()
    return row
