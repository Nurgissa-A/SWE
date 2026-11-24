"""Retain external operator actions without fabricating a user identity."""

import sqlalchemy as sa

from alembic import op

revision = "0004_operator_audit"
down_revision = "0003_sessions"
branch_labels = None
depends_on = None


def upgrade():
    op.alter_column("platform_admin_audit", "actor_id", nullable=True)
    op.add_column(
        "platform_admin_audit",
        sa.Column("actor_kind", sa.String(16), nullable=False, server_default="account"),
    )
    op.alter_column("platform_admin_audit", "actor_kind", server_default=None)
    op.create_check_constraint(
        "ck_audit_actor_kind", "platform_admin_audit", "actor_kind IN ('account','operator')"
    )


def downgrade():
    if op.get_bind().scalar(
        sa.text("SELECT count(*) FROM platform_admin_audit WHERE actor_id IS NULL")
    ):
        raise RuntimeError("Refusing downgrade that would discard retained operator audit history")
    op.drop_constraint("ck_audit_actor_kind", "platform_admin_audit", type_="check")
    op.drop_column("platform_admin_audit", "actor_kind")
    op.alter_column("platform_admin_audit", "actor_id", nullable=False)
