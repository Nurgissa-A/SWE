"""Python-owned validated image pointers; Go remains Event/visibility authority."""

import sqlalchemy as sa

from alembic import op

revision = "0005_event_images"
down_revision = "0004_operator_audit"
branch_labels = None
depends_on = None


def upgrade():
    op.create_table(
        "platform_event_images",
        sa.Column("event_id", sa.Uuid(), primary_key=True),
        sa.Column(
            "created_by",
            sa.Uuid(),
            sa.ForeignKey("identity_users.id", ondelete="RESTRICT"),
            nullable=False,
        ),
        sa.Column("sha256", sa.String(64), nullable=False),
        sa.Column("width", sa.Integer(), nullable=False),
        sa.Column("height", sa.Integer(), nullable=False),
        sa.Column("updated_at", sa.DateTime(timezone=True), nullable=False),
        sa.CheckConstraint(
            "width > 0 AND height > 0 AND width <= 4096 AND height <= 4096 "
            "AND width * height <= 8000000",
            name="ck_image_dimensions",
        ),
    )
    op.create_index("ix_platform_event_images_sha256", "platform_event_images", ["sha256"])


def downgrade():
    op.drop_table("platform_event_images")
