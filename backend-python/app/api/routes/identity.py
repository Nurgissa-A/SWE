from uuid import UUID

from fastapi import APIRouter, Depends, HTTPException, Query, Request, Response
from sqlalchemy import or_, select
from sqlalchemy.exc import IntegrityError
from sqlalchemy.orm import Session

from app.api.dependencies import current_user, require_roles
from app.db.session import get_session
from app.models.identity import OrganizerProfile, Role, User
from app.repositories.identity import IdentityRepository
from app.schemas.identity import (
    AdminUser,
    Credentials,
    Login,
    LoginResponse,
    ProfileInput,
    ProfileResponse,
    ResetPassword,
    ResetRequest,
    StaffAccount,
    UserResponse,
    UserStatusUpdate,
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
    for field, value in data.model_dump(exclude_unset=True).items():
        setattr(user, field, value)
    session.commit()
    return user


@router.get("/users/lookup", response_model=StaffAccount)
def lookup_staff_account(
    email: str = Query(min_length=3, max_length=254),
    actor: User = Depends(require_roles(Role.ORGANIZER, Role.PLATFORM_ADMIN)),
    session: Session = Depends(get_session),
):
    user = session.scalar(
        select(User).where(
            User.email == email.strip().lower(),
            User.email_verified.is_(True),
            User.is_active.is_(True),
        )
    )
    if user is None:
        raise HTTPException(404, "No verified active account uses this email")
    return user


@router.get("/admin/users", response_model=list[AdminUser])
def admin_users(
    q: str = Query(default="", max_length=254),
    limit: int = Query(default=100, ge=1, le=100),
    offset: int = Query(default=0, ge=0),
    actor: User = Depends(require_roles(Role.PLATFORM_ADMIN)),
    session: Session = Depends(get_session),
):
    query = select(User)
    if term := q.strip():
        query = query.where(
            or_(
                User.email.icontains(term, autoescape=True),
                User.display_name.icontains(term, autoescape=True),
            )
        )
    return session.scalars(
        query.order_by(User.created_at.desc(), User.id).offset(offset).limit(limit)
    ).all()


@router.patch("/admin/users/{user_id}", response_model=AdminUser)
def update_user_status(
    user_id: UUID,
    data: UserStatusUpdate,
    actor: User = Depends(require_roles(Role.PLATFORM_ADMIN)),
    session: Session = Depends(get_session),
):
    user = session.get(User, user_id, with_for_update=True, populate_existing=True)
    if user is None:
        raise HTTPException(404, "Account not found")
    if user.role == Role.PLATFORM_ADMIN:
        raise HTTPException(403, "Platform administrator status is managed by operators")
    if user.is_active != data.is_active:
        user.is_active = data.is_active
        user.token_version += 1
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
