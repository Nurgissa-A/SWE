"""Host/operator-only role management; deliberately no public privileged-role endpoint."""

import argparse

from sqlalchemy import select
from sqlalchemy.orm import Session

from app.core.config import Settings
from app.db.session import make_engine
from app.models.identity import Role, User


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=["set-role", "disable-user"])
    parser.add_argument("email")
    parser.add_argument("--role", choices=[r.value for r in Role])
    args = parser.parse_args()
    if args.command == "set-role" and not args.role:
        parser.error("set-role requires --role")
    engine = make_engine(Settings().database_url)
    try:
        with Session(engine) as session:
            user = session.scalar(
                select(User).where(User.email == args.email.strip().lower()).with_for_update()
            )
            if user is None or not user.email_verified:
                parser.error("A verified existing user is required")
            if args.command == "set-role":
                user.role = args.role
            else:
                user.is_active = False
            user.token_version += 1
            session.commit()
            print(f"Updated user {user.id}; existing access tokens invalidated.")
    finally:
        engine.dispose()


if __name__ == "__main__":
    main()
