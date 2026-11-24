from collections.abc import Generator

from fastapi import Request
from sqlalchemy import create_engine
from sqlalchemy.orm import DeclarativeBase, Session, sessionmaker

from app.core.config import Settings


class Base(DeclarativeBase):
    pass


def make_engine(url: str, settings: Settings | None = None):
    settings = settings or Settings()
    return create_engine(
        url,
        pool_pre_ping=True,
        pool_size=5,
        max_overflow=5,
        pool_timeout=settings.db_pool_timeout_seconds,
        hide_parameters=True,
        connect_args={
            "connect_timeout": settings.db_connect_timeout_seconds,
            "options": (
                f"-c statement_timeout={settings.db_statement_timeout_ms} "
                f"-c lock_timeout={settings.db_lock_timeout_ms}"
            ),
        },
    )


def make_session_factory(engine):
    return sessionmaker(bind=engine, expire_on_commit=False)


def get_session(request: Request) -> Generator[Session, None, None]:
    with request.app.state.session_factory() as session:
        yield session
