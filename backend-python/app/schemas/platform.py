from datetime import datetime
from typing import Literal
from uuid import UUID

from pydantic import Field, field_validator

from app.schemas.identity import APIModel, UserResponse

Category = Literal[
    "ticket_delivery",
    "payment",
    "refund",
    "seating",
    "event_information",
    "check_in",
    "account",
    "technical",
]
CaseStatus = Literal["open", "in_progress", "waiting_for_customer", "resolved"]


class CaseInput(APIModel):
    subject: str = Field(min_length=1, max_length=160)
    message: str = Field(min_length=1, max_length=4000)
    category: Category
    event_id: UUID | None = None
    order_id: UUID | None = None
    ticket_id: UUID | None = None

    @field_validator("subject", "message")
    @classmethod
    def nonblank(cls, value):
        if not value.strip():
            raise ValueError("Text cannot be blank")
        return value.strip()


class MessageInput(APIModel):
    body: str = Field(min_length=1, max_length=4000)

    @field_validator("body")
    @classmethod
    def nonblank(cls, value):
        return CaseInput.nonblank(value)


class CaseChange(APIModel):
    status: CaseStatus | None = None
    assignee_id: UUID | None = None
    escalated: bool | None = None


class CaseResponse(APIModel):
    id: UUID
    requester_id: UUID
    organizer_id: UUID | None
    assignee_id: UUID | None
    event_id: UUID | None
    order_id: UUID | None
    ticket_id: UUID | None
    channel: Literal["event", "platform"]
    category: Category
    subject: str
    status: CaseStatus
    sequence: int
    created_at: datetime
    updated_at: datetime


class MessageResponse(APIModel):
    id: UUID
    case_id: UUID
    author_id: UUID
    sequence: int
    body: str
    created_at: datetime


class BusinessNotification(APIModel):
    source_id: UUID
    user_id: UUID
    template: Literal[
        "order_confirmation",
        "ticket_delivery",
        "payment_failed",
        "event_updated",
        "event_cancelled",
        "refund_completed",
        "payout_status",
    ]
    event_id: UUID | None = None
    order_id: UUID | None = None
    ticket_id: UUID | None = None
    simulation: Literal[True] = True


class SettingsInput(APIModel):
    activation_fee_minor: int = Field(ge=0, le=100000000, strict=True)
    expected_version: int = Field(ge=1, strict=True)


class SuspensionInput(APIModel):
    is_active: bool = Field(strict=True)


class CasePage(APIModel):
    items: list[CaseResponse]
    next_cursor: UUID | None


class MessagePage(APIModel):
    items: list[MessageResponse]
    next_cursor: int


class NotificationResponse(APIModel):
    id: int
    template: str
    title: str
    context: dict[str, str]
    link: str
    state: Literal["pending", "sending", "delivered", "failed", "skipped"]
    attempts: int
    read_at: datetime | None
    created_at: datetime


class NotificationPage(APIModel):
    items: list[NotificationResponse]
    next_cursor: int


class JobResponse(APIModel):
    id: int
    source: str
    template: str
    state: Literal["pending", "sending", "delivered", "failed", "skipped"]
    attempts: int
    failure_code: str | None
    next_attempt_at: datetime


class JobPage(APIModel):
    items: list[JobResponse]
    next_cursor: int


class EnqueuedResponse(APIModel):
    id: int
    state: Literal["pending", "sending", "delivered", "failed", "skipped"]


class SettingsResponse(APIModel):
    activation_fee_minor: int
    currency: Literal["KZT"]
    version: int
    simulation_only: Literal[True]
    updated_at: datetime


class AuditResponse(APIModel):
    id: int
    actor_id: UUID | None
    actor_kind: Literal["account", "operator"]
    action: str
    target_id: str
    detail: str
    created_at: datetime


class AuditPage(APIModel):
    items: list[AuditResponse]
    next_cursor: int


class SupportActionResponse(APIModel):
    id: UUID
    actor_id: UUID
    action: str
    detail: str
    created_at: datetime


class AdminUserResponse(UserResponse):
    is_active: bool


class UserPage(APIModel):
    items: list[AdminUserResponse]
    next_cursor: UUID | None


class SupportHistoryPage(APIModel):
    items: list[SupportActionResponse]
    next_cursor: UUID | None
