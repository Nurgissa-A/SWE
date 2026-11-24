"""Run durable delivery jobs: python -m app.worker [--once]."""

import argparse
import logging
import signal
import threading
from datetime import UTC, datetime

from sqlalchemy import delete, text

from app.core.config import Settings
from app.db.session import make_engine, make_session_factory
from app.models.platform import RateLimit, RevokedToken
from app.services.email import SMTPEmailSender
from app.services.notifications import Notifications


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--once", action="store_true")
    parser.add_argument("--healthcheck", action="store_true")
    args = parser.parse_args()
    settings = Settings()
    engine = make_engine(settings.database_url, settings)
    if args.healthcheck:
        try:
            with engine.connect() as conn:
                conn.execute(text("SELECT 1 FROM platform_notifications LIMIT 1"))
        finally:
            engine.dispose()
        return
    notifier = Notifications(settings.notification_key_file.read_bytes(), settings.frontend_url)
    stop = threading.Event()
    for sig in (signal.SIGINT, signal.SIGTERM):
        signal.signal(sig, lambda *_: stop.set())
    try:
        while not stop.is_set():
            try:
                notifier.drain(
                    make_session_factory(engine),
                    SMTPEmailSender(settings),
                    limit=50 if args.once else 1,
                )
                with engine.begin() as connection:
                    connection.execute(
                        delete(RateLimit).where(RateLimit.expires_at < datetime.now(UTC))
                    )
                    connection.execute(
                        delete(RevokedToken).where(RevokedToken.expires_at < datetime.now(UTC))
                    )
            except Exception:
                logging.getLogger("biletflow.worker").error("Notification storage unavailable")
                if args.once:
                    raise SystemExit(1) from None
            if args.once:
                break
            stop.wait(2)
    finally:
        engine.dispose()


if __name__ == "__main__":
    main()
