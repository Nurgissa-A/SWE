from sqlalchemy import create_engine

from alembic import context
from app.core.config import Settings
from app.db.session import Base
from app.models import identity  # noqa: F401

config = context.config


def run_migrations():
    engine = create_engine(Settings().database_url)
    with engine.connect() as conn:
        context.configure(connection=conn, target_metadata=Base.metadata, compare_type=True)
        with context.begin_transaction():
            context.run_migrations()
    engine.dispose()


run_migrations()
