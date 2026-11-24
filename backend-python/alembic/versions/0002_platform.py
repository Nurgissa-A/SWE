"""Python-owned notification, support and platform administration storage."""

import sqlalchemy as sa

from alembic import op

revision = "0002_platform"
down_revision = "0001_identity"
branch_labels = None
depends_on = None


def upgrade():
    # Fixed revision-owned table definitions: later ORM changes cannot rewrite this migration.
    op.create_table(
        "platform_notifications",
        sa.Column("id", sa.BigInteger(), sa.Identity(), primary_key=True),
        sa.Column("source", sa.String(24), nullable=False),
        sa.Column("source_id", sa.Uuid(), nullable=False),
        sa.Column(
            "user_id",
            sa.Uuid(),
            sa.ForeignKey("identity_users.id", ondelete="CASCADE"),
            nullable=False,
        ),
        sa.Column("template", sa.String(40), nullable=False),
        sa.Column("encrypted_payload", sa.Text(), nullable=False),
        sa.Column("payload_hash", sa.String(64), nullable=False),
        sa.Column(
            "action_token_id",
            sa.Uuid(),
            sa.ForeignKey("identity_action_tokens.id", ondelete="CASCADE"),
        ),
        sa.Column("state", sa.String(16), nullable=False),
        sa.Column("attempts", sa.Integer(), nullable=False),
        sa.Column("next_attempt_at", sa.DateTime(timezone=True), nullable=False),
        sa.Column("lease_until", sa.DateTime(timezone=True)),
        sa.Column("lease_id", sa.Uuid()),
        sa.Column("delivered_at", sa.DateTime(timezone=True)),
        sa.Column("read_at", sa.DateTime(timezone=True)),
        sa.Column("failure_code", sa.String(32)),
        sa.Column("created_at", sa.DateTime(timezone=True), nullable=False),
        sa.UniqueConstraint(
            "source", "source_id", "user_id", "template", name="uq_notification_action"
        ),
        sa.CheckConstraint(
            "state IN ('pending','sending','delivered','failed','skipped')",
            name="ck_notification_state",
        ),
        sa.CheckConstraint("attempts >= 0", name="ck_notification_attempts"),
    )
    op.create_index("ix_notification_due", "platform_notifications", ["state", "next_attempt_at"])
    op.create_index("ix_notification_user_cursor", "platform_notifications", ["user_id", "id"])
    op.create_table(
        "platform_support_cases",
        sa.Column("id", sa.Uuid(), primary_key=True),
        sa.Column(
            "requester_id",
            sa.Uuid(),
            sa.ForeignKey("identity_users.id", ondelete="RESTRICT"),
            nullable=False,
        ),
        sa.Column("organizer_id", sa.Uuid()),
        sa.Column(
            "assignee_id", sa.Uuid(), sa.ForeignKey("identity_users.id", ondelete="RESTRICT")
        ),
        sa.Column("event_id", sa.Uuid()),
        sa.Column("order_id", sa.Uuid()),
        sa.Column("ticket_id", sa.Uuid()),
        sa.Column("channel", sa.String(16), nullable=False),
        sa.Column("category", sa.String(24), nullable=False),
        sa.Column("subject", sa.String(160), nullable=False),
        sa.Column("status", sa.String(24), nullable=False),
        sa.Column("request_key", sa.Uuid(), nullable=False),
        sa.Column("request_hash", sa.String(64), nullable=False),
        sa.Column("sequence", sa.Integer(), nullable=False),
        sa.Column("created_at", sa.DateTime(timezone=True), nullable=False),
        sa.Column("updated_at", sa.DateTime(timezone=True), nullable=False),
        sa.UniqueConstraint("requester_id", "request_key", name="uq_case_request"),
        sa.CheckConstraint("channel IN ('event','platform')", name="ck_case_channel"),
        sa.CheckConstraint(
            "status IN ('open','in_progress','waiting_for_customer','resolved')",
            name="ck_case_status",
        ),
        sa.CheckConstraint("sequence >= 1", name="ck_case_sequence"),
    )
    for name, cols in [
        ("requester", ["requester_id", "id"]),
        ("event", ["event_id", "updated_at"]),
        ("assignee", ["assignee_id", "updated_at"]),
    ]:
        op.create_index("ix_case_" + name, "platform_support_cases", cols)
    op.create_table(
        "platform_support_messages",
        sa.Column("id", sa.Uuid(), primary_key=True),
        sa.Column(
            "case_id",
            sa.Uuid(),
            sa.ForeignKey("platform_support_cases.id", ondelete="RESTRICT"),
            nullable=False,
        ),
        sa.Column(
            "author_id",
            sa.Uuid(),
            sa.ForeignKey("identity_users.id", ondelete="RESTRICT"),
            nullable=False,
        ),
        sa.Column("sequence", sa.Integer(), nullable=False),
        sa.Column("body", sa.Text(), nullable=False),
        sa.Column("request_key", sa.Uuid(), nullable=False),
        sa.Column("created_at", sa.DateTime(timezone=True), nullable=False),
        sa.UniqueConstraint("case_id", "sequence", name="uq_message_sequence"),
        sa.UniqueConstraint("case_id", "author_id", "request_key", name="uq_message_request"),
        sa.CheckConstraint("length(btrim(body)) > 0", name="ck_message_body"),
    )
    op.create_table(
        "platform_support_actions",
        sa.Column("id", sa.Uuid(), primary_key=True),
        sa.Column(
            "case_id",
            sa.Uuid(),
            sa.ForeignKey("platform_support_cases.id", ondelete="RESTRICT"),
            nullable=False,
        ),
        sa.Column(
            "actor_id",
            sa.Uuid(),
            sa.ForeignKey("identity_users.id", ondelete="RESTRICT"),
            nullable=False,
        ),
        sa.Column("request_key", sa.Uuid(), nullable=False),
        sa.Column("request_hash", sa.String(64), nullable=False),
        sa.Column("action", sa.String(32), nullable=False),
        sa.Column("detail", sa.String(160), nullable=False),
        sa.Column("created_at", sa.DateTime(timezone=True), nullable=False),
        sa.UniqueConstraint("case_id", "actor_id", "request_key", name="uq_support_action"),
    )
    op.create_table(
        "platform_settings",
        sa.Column("id", sa.Integer(), primary_key=True),
        sa.Column("activation_fee_minor", sa.Integer(), nullable=False),
        sa.Column("version", sa.Integer(), nullable=False),
        sa.Column("updated_at", sa.DateTime(timezone=True), nullable=False),
        sa.CheckConstraint("id = 1", name="ck_platform_singleton"),
        sa.CheckConstraint(
            "activation_fee_minor >= 0 AND activation_fee_minor <= 100000000",
            name="ck_activation_fee",
        ),
    )
    op.execute("INSERT INTO platform_settings VALUES(1,0,1,now())")
    op.create_table(
        "platform_admin_audit",
        sa.Column("id", sa.BigInteger(), sa.Identity(), primary_key=True),
        sa.Column(
            "actor_id",
            sa.Uuid(),
            sa.ForeignKey("identity_users.id", ondelete="RESTRICT"),
            nullable=False,
        ),
        sa.Column("action", sa.String(40), nullable=False),
        sa.Column("target_id", sa.String(64), nullable=False),
        sa.Column("detail", sa.String(160), nullable=False),
        sa.Column("created_at", sa.DateTime(timezone=True), nullable=False),
    )
    op.create_table(
        "identity_rate_limits",
        sa.Column("key", sa.String(64), primary_key=True),
        sa.Column("count", sa.Integer(), nullable=False),
        sa.Column("expires_at", sa.DateTime(timezone=True), nullable=False),
    )
    op.create_index("ix_identity_rate_limits_expires_at", "identity_rate_limits", ["expires_at"])


def downgrade():
    for name in [
        "identity_rate_limits",
        "platform_admin_audit",
        "platform_settings",
        "platform_support_actions",
        "platform_support_messages",
        "platform_support_cases",
        "platform_notifications",
    ]:
        op.drop_table(name)
