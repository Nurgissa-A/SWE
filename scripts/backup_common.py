"""Private restore proof helpers; prints aggregate fingerprints, never user data."""

import json
import subprocess
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
TABLES = [
    "identity_users",
    "identity_action_tokens",
    "identity_email_deliveries",
    "identity_revoked_tokens",
    "platform_support_actions",
    "platform_event_images",
    "identity_organizer_profiles",
    "platform_support_cases",
    "platform_support_messages",
    "platform_notifications",
    "platform_admin_audit",
    "platform_settings",
]


def execute(*args, input=None):
    return subprocess.run(
        ["docker", "compose", "exec", "-T", "postgres", *args],
        cwd=ROOT,
        input=input,
        capture_output=True,
        check=True,
    ).stdout


def snapshot(database):
    fields = []
    for table in TABLES:
        query = (
            "json_build_object('count',count(*),'fingerprint',"
            "md5(coalesce(string_agg(row_to_json(t)::text,'' ORDER BY row_to_json(t)::text),'')))"
        )
        fields.append("'" + table + "',(SELECT " + query + " FROM " + table + " t)")
    query = (
        "SELECT json_build_object("
        + ",".join(fields)
        + ",'revision',(SELECT version_num FROM alembic_version))"
    )
    return json.loads(execute("psql", "-U", "biletflow", "-d", database, "-Atc", query))
