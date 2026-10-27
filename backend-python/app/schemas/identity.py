from datetime import datetime
from uuid import UUID

from pydantic import BaseModel, ConfigDict, EmailStr, Field, field_validator
from pydantic.alias_generators import to_camel

from app.models.identity import Role


class APIModel(BaseModel):
    model_config = ConfigDict(
        alias_generator=to_camel, populate_by_name=True, from_attributes=True, extra="forbid"
    )


class Credentials(APIModel):
    email: EmailStr = Field(max_length=254)
    password: str = Field(min_length=8, max_length=1024)

    @field_validator("email", mode="before")
    @classmethod
    def normalize_email(cls, value):
        return value.strip().lower() if isinstance(value, str) else value


class Login(Credentials):
    password: str = Field(min_length=1, max_length=1024)


class VerifyToken(APIModel):
    token: str = Field(min_length=1, max_length=256)


class ResetRequest(APIModel):
    email: EmailStr = Field(max_length=254)

    @field_validator("email", mode="before")
    @classmethod
    def normalize_email(cls, value):
        return value.strip().lower() if isinstance(value, str) else value


class ResetPassword(VerifyToken):
    new_password: str = Field(min_length=8, max_length=1024)


class UserResponse(APIModel):
    id: UUID
    email: EmailStr
    role: Role
    display_name: str | None
    email_verified: bool
    created_at: datetime


class LoginResponse(APIModel):
    token: str
    user: UserResponse


class UserUpdate(APIModel):
    display_name: str = Field(max_length=100)


class ProfileInput(APIModel):
    contact_email: EmailStr = Field(max_length=254)
    contact_phone: str | None = Field(default=None, max_length=32, pattern=r"^[+0-9 ()-]*$")
    payout_account_placeholder: str | None = Field(default=None, max_length=100)

    @field_validator("payout_account_placeholder")
    @classmethod
    def simulation_only(cls, value):
        if value is not None and not value.startswith("demo:"):
            raise ValueError("Only demo: prefixed payout placeholders are accepted")
        return value


class ProfileResponse(APIModel):
    id: UUID
    user_id: UUID
    contact_email: EmailStr
    contact_phone: str | None
    payout_account_placeholder: str | None
