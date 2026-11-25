"""Persist account language preference."""

import sqlalchemy as sa

from alembic import op

revision = "0002_account_locale"
down_revision = "0001_identity"
branch_labels = None
depends_on = None


def upgrade():
    op.add_column(
        "identity_users",
        sa.Column("locale", sa.String(2), nullable=False, server_default="en"),
    )
    op.create_check_constraint("ck_user_locale", "identity_users", "locale IN ('kk','ru','en')")


def downgrade():
    op.drop_constraint("ck_user_locale", "identity_users", type_="check")
    op.drop_column("identity_users", "locale")
