import os
from pathlib import Path

import pytest
from alembic.config import Config
from sqlalchemy import create_engine, inspect, select
from sqlalchemy.orm import Session

from alembic import command
from app.models.identity import User


@pytest.mark.integration
def test_migration_and_persistence(monkeypatch):
    url = os.getenv("TEST_DATABASE_URL")
    if not url:
        pytest.skip("Set TEST_DATABASE_URL to a disposable PostgreSQL database")
    monkeypatch.setenv("DATABASE_URL", url)
    config = Config(str(Path(__file__).parents[1] / "alembic.ini"))
    command.upgrade(config, "head")
    engine = create_engine(url)
    try:
        assert "identity_users" in inspect(engine).get_table_names()
        with Session(engine) as session:
            row = User(email="persistence@example.com", password_hash="test-hash")
            session.add(row)
            session.commit()
            uid = row.id
        with Session(engine) as session:
            saved = session.scalar(select(User).where(User.id == uid))
            assert saved.email == "persistence@example.com"
            session.delete(saved)
            session.commit()
    finally:
        engine.dispose()
