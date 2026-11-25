from pathlib import Path

from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", extra="ignore")

    environment: str = "development"
    database_url: str = "postgresql+psycopg://localhost/biletflow"
    db_pool_timeout_seconds: int = Field(default=5, ge=1, le=60)
    db_connect_timeout_seconds: int = Field(default=5, ge=1, le=60)
    db_statement_timeout_ms: int = Field(default=10000, ge=100, le=60000)
    db_lock_timeout_ms: int = Field(default=3000, ge=100, le=60000)
    jwt_private_key_file: Path = Path(".secrets/jwt-private.pem")
    jwt_public_key_file: Path = Path(".secrets/jwt-public.pem")
    jwt_key_id: str = "biletflow-local-1"
    jwt_issuer: str = "biletflow-identity"
    jwt_audience: str = "biletflow-api"
    access_token_minutes: int = Field(default=15, ge=1, le=60)
    verification_token_minutes: int = Field(default=60, ge=1, le=1440)
    reset_token_minutes: int = Field(default=15, ge=1, le=60)
    smtp_host: str = "localhost"
    smtp_port: int = 1025
    smtp_starttls: bool = False
    smtp_username: str | None = None
    smtp_password: str | None = None
    email_from: str = "noreply@biletflow.example"
    frontend_url: str = "http://localhost:3000"
    events_api_url: str = "http://localhost:8080"
    cors_origins: list[str] = ["http://localhost:3000"]
    trusted_hosts: list[str] = ["localhost", "127.0.0.1", "python", "testserver"]
