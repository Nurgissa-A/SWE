"""Bootstrap, grant and verify separate runtime DB roles using migration credentials."""

import argparse
import os

import psycopg
from psycopg import sql
from sqlalchemy.engine import make_url

from app.core.config import Settings

ROLES = {
    "biletflow_identity": "IDENTITY_DATABASE_PASSWORD",
    "biletflow_notifier": "NOTIFIER_DATABASE_PASSWORD",
    "biletflow_events": "EVENTS_DATABASE_PASSWORD",
}


def connect(role=None):
    url = make_url(Settings().database_url).set(drivername="postgresql")
    if role:
        url = url.set(username=role, password=os.environ[ROLES[role]])
    return psycopg.connect(url.render_as_string(hide_password=False), connect_timeout=5)


def bootstrap():
    with connect() as connection:
        for name, key in ROLES.items():
            password = os.environ[key]
            if len(password) < 32:
                raise ValueError("Runtime database passwords must have at least 32 characters")
            if not connection.execute(
                "SELECT 1 FROM pg_roles WHERE rolname=%s", (name,)
            ).fetchone():
                connection.execute(sql.SQL("CREATE ROLE {} LOGIN").format(sql.Identifier(name)))
            connection.execute(
                sql.SQL(
                    "ALTER ROLE {} LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE "
                    "NOREPLICATION NOINHERIT PASSWORD {}"
                ).format(sql.Identifier(name), sql.Literal(password))
            )
        connection.execute("REVOKE CREATE ON SCHEMA public FROM PUBLIC")
        # Namespace only. Go authors all Event/business tables and migrations.
        connection.execute("CREATE SCHEMA IF NOT EXISTS events")
        connection.execute("GRANT USAGE ON SCHEMA events TO biletflow_events")
        connection.execute(
            "ALTER DEFAULT PRIVILEGES IN SCHEMA events "
            "GRANT SELECT,INSERT,UPDATE,DELETE ON TABLES TO biletflow_events"
        )
        connection.execute(
            "ALTER DEFAULT PRIVILEGES IN SCHEMA events "
            "GRANT USAGE,SELECT ON SEQUENCES TO biletflow_events"
        )


def grant():
    with connect() as connection:
        tables = connection.execute(
            "SELECT tablename FROM pg_tables WHERE schemaname='public'"
        ).fetchall()
        for (name,) in tables:
            if name.startswith(("identity_", "platform_")):
                connection.execute(
                    sql.SQL(
                        "GRANT SELECT,INSERT,UPDATE,DELETE ON TABLE {} TO biletflow_identity"
                    ).format(sql.Identifier(name))
                )
                connection.execute(
                    sql.SQL(
                        "REVOKE ALL ON TABLE {} FROM biletflow_events,biletflow_notifier"
                    ).format(sql.Identifier(name))
                )
        connection.execute(
            "REVOKE UPDATE,DELETE ON platform_admin_audit,platform_support_actions,"
            "platform_support_messages FROM biletflow_identity"
        )
        connection.execute("GRANT SELECT ON alembic_version TO biletflow_identity")
        # All current public sequences belong to Python identity/platform migrations.
        for (name,) in connection.execute(
            "SELECT sequencename FROM pg_sequences WHERE schemaname='public'"
        ).fetchall():
            if name.startswith(("identity_", "platform_")):
                connection.execute(
                    sql.SQL("GRANT USAGE,SELECT ON SEQUENCE {} TO biletflow_identity").format(
                        sql.Identifier(name)
                    )
                )
        connection.execute(
            "GRANT SELECT(id,email,email_verified,is_active) "
            "ON identity_users TO biletflow_notifier"
        )
        connection.execute(
            "GRANT SELECT(id,user_id,used_at,expires_at) "
            "ON identity_action_tokens TO biletflow_notifier"
        )
        connection.execute("GRANT SELECT,UPDATE ON platform_notifications TO biletflow_notifier")
        connection.execute(
            "GRANT SELECT,INSERT,UPDATE ON identity_email_deliveries TO biletflow_notifier"
        )
        connection.execute(
            "GRANT SELECT,DELETE ON identity_rate_limits,identity_revoked_tokens "
            "TO biletflow_notifier"
        )
        for (name,) in connection.execute(
            "SELECT tablename FROM pg_tables WHERE schemaname='events'"
        ).fetchall():
            permission = "SELECT" if name == "schema_migrations" else "SELECT,INSERT,UPDATE,DELETE"
            connection.execute(
                sql.SQL("GRANT " + permission + " ON TABLE events.{} TO biletflow_events").format(
                    sql.Identifier(name)
                )
            )
            if name == "schema_migrations":
                connection.execute(
                    "REVOKE INSERT,UPDATE,DELETE ON events.schema_migrations FROM biletflow_events"
                )


def check():
    for role in ROLES:
        with connect(role) as connection:
            assert connection.execute("SELECT current_user").fetchone()[0] == role
            assert not connection.execute(
                "SELECT has_schema_privilege(current_user,'public','CREATE')"
            ).fetchone()[0]
            if role == "biletflow_identity":
                connection.execute("SELECT 1 FROM identity_users LIMIT 1")
                connection.execute("SELECT 1 FROM platform_support_cases LIMIT 1")
            else:
                try:
                    connection.execute("SELECT password_hash FROM identity_users LIMIT 1")
                except psycopg.errors.InsufficientPrivilege:
                    connection.rollback()
                else:
                    raise AssertionError("Runtime role could read password hashes")
            if role == "biletflow_notifier":
                connection.execute("SELECT 1 FROM platform_notifications LIMIT 1")
            if role == "biletflow_events":
                assert connection.execute(
                    "SELECT has_schema_privilege(current_user,'events','USAGE')"
                ).fetchone()[0]
    print("Runtime database ACL checks passed: separate identity, notifier and events roles.")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("operation", choices=["bootstrap", "grant", "check"])
    args = parser.parse_args()
    {"bootstrap": bootstrap, "grant": grant, "check": check}[args.operation]()


if __name__ == "__main__":
    main()
