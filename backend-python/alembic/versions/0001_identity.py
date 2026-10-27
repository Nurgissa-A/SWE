"""Create Python-owned identity tables. Never creates Go-owned Event tables."""

import sqlalchemy as sa

from alembic import op

revision = "0001_identity"
down_revision = None
branch_labels = None
depends_on = None


def upgrade():
    op.create_table(
        "identity_users",
        sa.Column("id", sa.Uuid(), primary_key=True),
        sa.Column("email", sa.String(254), nullable=False, unique=True),
        sa.Column("password_hash", sa.String(512), nullable=False),
        sa.Column("role", sa.String(32), nullable=False),
        sa.Column("display_name", sa.String(100)),
        sa.Column("email_verified", sa.Boolean(), nullable=False),
        sa.Column("is_active", sa.Boolean(), nullable=False),
        sa.Column("token_version", sa.Integer(), nullable=False),
        sa.Column("created_at", sa.DateTime(timezone=True), nullable=False),
        sa.CheckConstraint(
            "role IN ('attendee','organizer','event_admin','platform_admin')", name="ck_user_role"
        ),
        sa.CheckConstraint("token_version >= 0", name="ck_token_version"),
        sa.CheckConstraint("email = lower(email)", name="ck_email_normalized"),
    )
    op.create_table(
        "identity_organizer_profiles",
        sa.Column("id", sa.Uuid(), primary_key=True),
        sa.Column(
            "user_id",
            sa.Uuid(),
            sa.ForeignKey("identity_users.id", ondelete="RESTRICT"),
            nullable=False,
            unique=True,
        ),
        sa.Column("contact_email", sa.String(254), nullable=False),
        sa.Column("contact_phone", sa.String(32)),
        sa.Column("payout_account_placeholder", sa.String(100)),
    )
    op.create_table(
        "identity_action_tokens",
        sa.Column("id", sa.Uuid(), primary_key=True),
        sa.Column(
            "user_id",
            sa.Uuid(),
            sa.ForeignKey("identity_users.id", ondelete="CASCADE"),
            nullable=False,
        ),
        sa.Column("token_hash", sa.String(64), nullable=False, unique=True),
        sa.Column("purpose", sa.String(32), nullable=False),
        sa.Column("expires_at", sa.DateTime(timezone=True), nullable=False),
        sa.Column("used_at", sa.DateTime(timezone=True)),
        sa.CheckConstraint("purpose IN ('verify_email','reset_password')", name="ck_token_purpose"),
    )
    op.create_index("ix_identity_action_tokens_user_id", "identity_action_tokens", ["user_id"])
    op.create_index(
        "ix_identity_action_tokens_expires_at", "identity_action_tokens", ["expires_at"]
    )
    op.create_table(
        "identity_email_deliveries",
        sa.Column("id", sa.Uuid(), primary_key=True),
        sa.Column(
            "action_token_id",
            sa.Uuid(),
            sa.ForeignKey("identity_action_tokens.id", ondelete="CASCADE"),
            nullable=False,
            unique=True,
        ),
        sa.Column("sent_at", sa.DateTime(timezone=True)),
        sa.Column("attempts", sa.Integer(), nullable=False),
    )


def downgrade():
    op.drop_table("identity_email_deliveries")
    op.drop_table("identity_action_tokens")
    op.drop_table("identity_organizer_profiles")
    op.drop_table("identity_users")
