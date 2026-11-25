"""Add session revocation, account audit and local event cover storage."""

import sqlalchemy as sa

from alembic import op

revision = "0003_identity_operations"
down_revision = "0002_account_locale"
branch_labels = None
depends_on = None


def upgrade():
    op.create_table(
        "identity_revoked_sessions",
        sa.Column("jti", sa.Uuid(), primary_key=True),
        sa.Column("expires_at", sa.DateTime(timezone=True), nullable=False),
    )
    op.create_index(
        "ix_identity_revoked_sessions_expires_at", "identity_revoked_sessions", ["expires_at"]
    )
    op.create_table(
        "identity_account_audit",
        sa.Column("id", sa.Uuid(), primary_key=True),
        sa.Column("actor_id", sa.Uuid(), nullable=False),
        sa.Column("account_id", sa.Uuid(), nullable=False),
        sa.Column("is_active", sa.Boolean(), nullable=False),
        sa.Column("created_at", sa.DateTime(timezone=True), nullable=False),
    )
    op.create_table(
        "identity_event_covers",
        sa.Column("event_id", sa.Uuid(), primary_key=True),
        sa.Column("body", sa.LargeBinary(), nullable=False),
    )


def downgrade():
    op.drop_table("identity_event_covers")
    op.drop_table("identity_account_audit")
    op.drop_table("identity_revoked_sessions")
