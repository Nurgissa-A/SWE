import hmac
from datetime import UTC, datetime
from uuid import UUID

from fastapi import APIRouter, Depends, Header, HTTPException, Query, Request, Response
from sqlalchemy import select, text, tuple_
from sqlalchemy.orm import Session

from app.api.dependencies import current_user, current_user_for_update, require_roles
from app.db.session import get_session
from app.models.identity import Role, User
from app.models.platform import (
    AdminAudit,
    Notification,
    PlatformSettings,
    SupportAction,
    SupportMessage,
)
from app.schemas.identity import UserResponse
from app.schemas.platform import (
    AdminUserResponse,
    AuditPage,
    BusinessNotification,
    CaseChange,
    CaseInput,
    CasePage,
    CaseResponse,
    EnqueuedResponse,
    JobPage,
    MessageInput,
    MessagePage,
    MessageResponse,
    NotificationPage,
    SettingsInput,
    SettingsResponse,
    SupportActionResponse,
    SupportHistoryPage,
    SuspensionInput,
    UserPage,
)
from app.services.support import SupportService

router = APIRouter(tags=["platform"])


def support(
    request: Request, user: User = Depends(current_user), session: Session = Depends(get_session)
):
    return SupportService(
        session,
        user,
        request.headers.get("Authorization", ""),
        request.state.request_id,
        request.app.state.business,
        request.app.state.notifications,
    )


def support_write(
    request: Request,
    user: User = Depends(current_user_for_update),
    session: Session = Depends(get_session),
):
    return SupportService(
        session,
        user,
        request.headers.get("Authorization", ""),
        request.state.request_id,
        request.app.state.business,
        request.app.state.notifications,
    )


def service_auth(request: Request):
    expected = "Bearer " + request.app.state.service_token
    supplied = request.headers.get("Authorization", "")
    if len(supplied) > 256 or not hmac.compare_digest(supplied.encode(), expected.encode()):
        raise HTTPException(
            401, "Invalid internal service credentials", headers={"WWW-Authenticate": "Bearer"}
        )


@router.post("/api/support/cases", response_model=CaseResponse, status_code=201)
def create_case(
    data: CaseInput,
    response: Response,
    key: UUID = Header(alias="Idempotency-Key"),
    svc: SupportService = Depends(support_write),
):
    row, created = svc.create(data, key)
    response.status_code = 201 if created else 200
    return row


@router.get("/api/support/cases", response_model=CasePage)
def list_cases(
    event_id: UUID | None = Query(None, alias="eventId"),
    after: UUID | None = None,
    limit: int = Query(50, ge=1, le=100),
    svc: SupportService = Depends(support),
):
    rows, cursor = svc.listing(event_id=event_id, after=after, limit=limit)
    return {
        "items": [
            CaseResponse.model_validate(row).model_dump(mode="json", by_alias=True) for row in rows
        ],
        "nextCursor": cursor,
    }


@router.get("/api/support/cases/{case_id}", response_model=CaseResponse)
def get_case(case_id: UUID, svc: SupportService = Depends(support)):
    return svc.case(case_id)


@router.get("/api/support/cases/{case_id}/messages", response_model=MessagePage)
def messages(
    case_id: UUID,
    after: int = Query(0, ge=0),
    limit: int = Query(50, ge=1, le=100),
    svc: SupportService = Depends(support),
):
    svc.case(case_id)
    rows = list(
        svc.session.scalars(
            select(SupportMessage)
            .where(SupportMessage.case_id == case_id, SupportMessage.sequence > after)
            .order_by(SupportMessage.sequence)
            .limit(limit)
        )
    )
    return {
        "items": [
            MessageResponse.model_validate(row).model_dump(mode="json", by_alias=True)
            for row in rows
        ],
        "nextCursor": rows[-1].sequence if rows else after,
    }


@router.post(
    "/api/support/cases/{case_id}/messages", response_model=MessageResponse, status_code=201
)
def reply(
    case_id: UUID,
    data: MessageInput,
    response: Response,
    key: UUID = Header(alias="Idempotency-Key"),
    svc: SupportService = Depends(support_write),
):
    row, created = svc.reply(case_id, data.body, key)
    response.status_code = 201 if created else 200
    return row


@router.patch("/api/support/cases/{case_id}", response_model=CaseResponse)
def change_case(
    case_id: UUID,
    data: CaseChange,
    key: UUID = Header(alias="Idempotency-Key"),
    svc: SupportService = Depends(support_write),
):
    return svc.change(case_id, data, key)


@router.get("/api/support/cases/{case_id}/history", response_model=SupportHistoryPage)
def case_history(
    case_id: UUID,
    after: UUID | None = None,
    limit: int = Query(50, ge=1, le=100),
    svc: SupportService = Depends(support),
):
    svc.case(case_id)
    query = select(SupportAction).where(SupportAction.case_id == case_id)
    if after:
        cursor = svc.session.get(SupportAction, after)
        if cursor is None or cursor.case_id != case_id:
            raise HTTPException(400, "Unknown history cursor")
        query = query.where(
            tuple_(SupportAction.created_at, SupportAction.id) > (cursor.created_at, cursor.id)
        )
    rows = list(
        svc.session.scalars(
            query.order_by(SupportAction.created_at, SupportAction.id).limit(limit + 1)
        )
    )
    return {
        "items": [
            SupportActionResponse.model_validate(row).model_dump(mode="json", by_alias=True)
            for row in rows[:limit]
        ],
        "nextCursor": rows[limit - 1].id if len(rows) > limit else None,
    }


@router.get("/api/notifications", response_model=NotificationPage)
def notifications(
    request: Request,
    after: int = Query(0, ge=0),
    limit: int = Query(50, ge=1, le=100),
    user: User = Depends(current_user),
    session: Session = Depends(get_session),
):
    rows = list(
        session.scalars(
            select(Notification)
            .where(
                Notification.user_id == user.id,
                Notification.source != "identity",
                Notification.id > after,
            )
            .order_by(Notification.id)
            .limit(limit)
        )
    )
    return {
        "items": [request.app.state.notifications.public(row) for row in rows],
        "nextCursor": rows[-1].id if rows else after,
    }


@router.patch("/api/notifications/{notification_id}/read", status_code=204)
def read_notification(
    notification_id: int,
    user: User = Depends(current_user_for_update),
    session: Session = Depends(get_session),
):
    row = session.scalar(
        select(Notification)
        .where(
            Notification.id == notification_id,
            Notification.user_id == user.id,
            Notification.source != "identity",
        )
        .with_for_update()
    )
    if row is None:
        raise HTTPException(404, "Notification not found")
    row.read_at = row.read_at or datetime.now(UTC)
    session.commit()


@router.post(
    "/internal/notifications",
    status_code=202,
    response_model=EnqueuedResponse,
    dependencies=[Depends(service_auth)],
)
def ingest_notification(
    data: BusinessNotification, request: Request, session: Session = Depends(get_session)
):
    user = session.get(User, data.user_id)
    if not user:
        raise HTTPException(404, "Recipient not found")
    payload = data.model_dump(mode="json", by_alias=True, exclude_none=True)
    for name in ["sourceId", "userId", "template"]:
        payload.pop(name, None)
    if data.template == "ticket_delivery" and not data.ticket_id:
        raise HTTPException(422, "ticketId required")
    row = request.app.state.notifications.enqueue(
        session, data.user_id, data.template, "business", data.source_id, payload
    )
    session.commit()
    return {"id": row.id, "state": row.state}


@router.get("/internal/organizer-profiles/{user_id}", dependencies=[Depends(service_auth)])
def internal_profile(user_id: UUID, session: Session = Depends(get_session)):
    from app.models.identity import OrganizerProfile

    user = session.get(User, user_id)
    row = session.scalar(select(OrganizerProfile).where(OrganizerProfile.user_id == user_id))
    if (
        not row
        or not user
        or not user.is_active
        or not user.email_verified
        or user.role != "organizer"
    ):
        raise HTTPException(404, "Active organizer profile not found")
    return {
        "userId": user.id,
        "contactEmail": row.contact_email,
        "contactPhone": row.contact_phone,
        "payoutAccountPlaceholder": row.payout_account_placeholder,
        "simulationOnly": True,
    }


def settings_response(row):
    return {
        "activationFeeMinor": row.activation_fee_minor,
        "currency": "KZT",
        "version": row.version,
        "simulationOnly": True,
        "updatedAt": row.updated_at,
    }


@router.get(
    "/internal/platform-settings",
    response_model=SettingsResponse,
    dependencies=[Depends(service_auth)],
)
def internal_settings(session: Session = Depends(get_session)):
    return settings_response(session.get(PlatformSettings, 1))


admin_role = require_roles(Role.PLATFORM_ADMIN)


def admin_write(session, user):
    # Global admin mutation order prevents two admins suspending one another in a cycle.
    version = user.token_version
    session.execute(text("SELECT pg_advisory_xact_lock(64721,2)"))
    session.refresh(user, with_for_update={"key_share": True})
    if not user.is_active or not user.email_verified or user.token_version != version:
        raise HTTPException(401, "Identity changed; sign in again")
    if user.role != "platform_admin":
        raise HTTPException(403, "Platform Admin required")


@router.get("/api/admin/users", response_model=UserPage)
def user_search(
    q: str = Query("", max_length=254),
    after: UUID | None = None,
    limit: int = Query(50, ge=1, le=100),
    user: User = Depends(admin_role),
    session: Session = Depends(get_session),
):
    query = select(User)
    if q:
        escaped = q.strip().lower().replace("\\", "\\\\").replace("%", "\\%").replace("_", "\\_")
        query = query.where(User.email.ilike("%" + escaped + "%", escape="\\"))
    if after:
        query = query.where(User.id > after)
    rows = list(session.scalars(query.order_by(User.id).limit(limit + 1)))
    return {
        "items": [
            {
                **UserResponse.model_validate(row).model_dump(mode="json", by_alias=True),
                "isActive": row.is_active,
            }
            for row in rows[:limit]
        ],
        "nextCursor": rows[limit - 1].id if len(rows) > limit else None,
    }


@router.patch("/api/admin/users/{user_id}", response_model=AdminUserResponse)
def suspend(
    user_id: UUID,
    data: SuspensionInput,
    user: User = Depends(admin_role),
    session: Session = Depends(get_session),
):
    admin_write(session, user)
    if user_id == user.id and not data.is_active:
        raise HTTPException(409, "Cannot suspend your own administrator account")
    target = session.get(User, user_id, with_for_update={"key_share": True}, populate_existing=True)
    if not target:
        raise HTTPException(404, "User not found")
    if target.is_active != data.is_active:
        target.is_active = data.is_active
        target.token_version += 1
        session.add(
            AdminAudit(
                actor_id=user.id,
                action="user_access",
                target_id=str(target.id),
                detail="enabled" if data.is_active else "suspended",
            )
        )
    session.commit()
    return target


@router.get("/api/admin/platform-settings", response_model=SettingsResponse)
def get_settings(user: User = Depends(admin_role), session: Session = Depends(get_session)):
    return settings_response(session.get(PlatformSettings, 1))


@router.patch("/api/admin/platform-settings", response_model=SettingsResponse)
def set_settings(
    data: SettingsInput, user: User = Depends(admin_role), session: Session = Depends(get_session)
):
    admin_write(session, user)
    row = session.get(PlatformSettings, 1, with_for_update=True, populate_existing=True)
    if row.version != data.expected_version:
        raise HTTPException(409, "Settings changed; reload before updating")
    row.activation_fee_minor = data.activation_fee_minor
    row.version += 1
    row.updated_at = datetime.now(UTC)
    session.add(
        AdminAudit(
            actor_id=user.id,
            action="activation_fee",
            target_id="platform",
            detail=f"KZT minor units {row.activation_fee_minor}; version {row.version}; simulation",
        )
    )
    session.commit()
    return settings_response(row)


@router.get("/api/admin/audit", response_model=AuditPage)
def admin_audit(
    after: int = Query(0, ge=0),
    limit: int = Query(50, ge=1, le=100),
    user: User = Depends(admin_role),
    session: Session = Depends(get_session),
):
    rows = list(
        session.scalars(
            select(AdminAudit).where(AdminAudit.id > after).order_by(AdminAudit.id).limit(limit)
        )
    )
    return {
        "items": [
            {
                "id": row.id,
                "actorId": row.actor_id,
                "actorKind": row.actor_kind,
                "action": row.action,
                "targetId": row.target_id,
                "detail": row.detail,
                "createdAt": row.created_at,
            }
            for row in rows
        ],
        "nextCursor": rows[-1].id if rows else after,
    }


@router.get("/api/admin/notification-jobs", response_model=JobPage)
def job_status(
    state: str | None = Query(None, pattern="^(pending|sending|delivered|failed|skipped)$"),
    after: int = Query(0, ge=0),
    limit: int = Query(50, ge=1, le=100),
    user: User = Depends(admin_role),
    session: Session = Depends(get_session),
):
    query = select(Notification).where(Notification.id > after)
    if state:
        query = query.where(Notification.state == state)
    rows = list(session.scalars(query.order_by(Notification.id).limit(limit)))
    return {
        "items": [
            {
                "id": row.id,
                "source": row.source,
                "template": row.template,
                "state": row.state,
                "attempts": row.attempts,
                "failureCode": row.failure_code,
                "nextAttemptAt": row.next_attempt_at,
            }
            for row in rows
        ],
        "nextCursor": rows[-1].id if rows else after,
    }


@router.post(
    "/api/admin/notification-jobs/{job_id}/retry", status_code=202, response_model=EnqueuedResponse
)
def retry_job(
    job_id: int, user: User = Depends(admin_role), session: Session = Depends(get_session)
):
    admin_write(session, user)
    row = session.get(Notification, job_id, with_for_update=True)
    if not row:
        raise HTTPException(404, "Notification not found")
    if row.state not in {"failed", "pending"}:
        raise HTTPException(409, "Notification is not retryable")
    row.state = "pending"
    row.next_attempt_at = datetime.now(UTC)
    row.attempts = 0
    row.failure_code = None
    session.add(
        AdminAudit(
            actor_id=user.id,
            action="notification_retry",
            target_id=str(row.id),
            detail="Delivery retry requested",
        )
    )
    session.commit()
    return {"id": row.id, "state": row.state}


@router.get("/api/admin/business/{resource}")
def business_search(
    resource: str,
    request: Request,
    q: str = Query("", max_length=100),
    user: User = Depends(admin_role),
):
    if resource not in {"events", "orders", "payments", "refunds", "activations"}:
        raise HTTPException(404, "Unknown business resource")
    try:
        return request.app.state.business.get(
            "/api/admin/" + resource,
            request.headers["Authorization"],
            request.state.request_id,
            {"q": q},
        )
    except HTTPException as exc:
        if exc.status_code == 404:
            raise HTTPException(503, "Go administration integration not available") from None
        raise


@router.post("/api/admin/business/events/{event_id}/suspend")
def suspend_event(event_id: UUID, request: Request, user: User = Depends(admin_role)):
    try:
        return request.app.state.business.request(
            "POST",
            f"/api/admin/business/events/{event_id}/suspend",
            request.headers["Authorization"],
            request.state.request_id,
        )
    except HTTPException as exc:
        if exc.status_code == 404:
            raise HTTPException(503, "Go event moderation integration not available") from None
        raise
