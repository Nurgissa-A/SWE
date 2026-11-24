"""Per-session logout revocation, independently enforced by live Go identity checks."""

import sqlalchemy as sa

from alembic import op

revision = "0003_sessions"
down_revision = "0002_platform"
branch_labels = None
depends_on = None


def upgrade():
    op.create_table(
        "identity_revoked_tokens",
        sa.Column("jti", sa.Uuid(), primary_key=True),
        sa.Column(
            "user_id",
            sa.Uuid(),
            sa.ForeignKey("identity_users.id", ondelete="CASCADE"),
            nullable=False,
        ),
        sa.Column("expires_at", sa.DateTime(timezone=True), nullable=False),
        sa.Column("revoked_at", sa.DateTime(timezone=True), nullable=False),
    )
    op.create_index(
        "ix_identity_revoked_tokens_expires_at", "identity_revoked_tokens", ["expires_at"]
    )


def downgrade():
    op.drop_table("identity_revoked_tokens")
