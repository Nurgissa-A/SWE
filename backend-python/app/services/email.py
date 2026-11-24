import smtplib
from email.message import EmailMessage
from typing import Protocol
from urllib.parse import urlencode

from app.core.config import Settings


class EmailSender(Protocol):
    def send(
        self, recipient: str, purpose: str, token: str, *, message_id: str | None = None
    ) -> None: ...

    def send_notification(
        self, recipient: str, subject: str, body: str, *, message_id: str
    ) -> None: ...


class SMTPEmailSender:
    def __init__(self, settings: Settings):
        self.settings = settings

    def send(
        self, recipient: str, purpose: str, token: str, *, message_id: str | None = None
    ) -> None:
        page = "verify-email" if purpose == "verify_email" else "reset-password"
        link = f"{self.settings.frontend_url.rstrip('/')}/{page}?{urlencode({'token': token})}"
        self.send_notification(
            recipient,
            f"BiletFlow {page.replace('-', ' ')}",
            f"Open this single-use link: {link}\nIf you did not request it, ignore it.",
            message_id=message_id or "<identity@biletflow>",
        )

    def send_notification(
        self, recipient: str, subject: str, body: str, *, message_id: str
    ) -> None:
        msg = EmailMessage()
        msg["From"], msg["To"], msg["Subject"] = self.settings.email_from, recipient, subject
        msg["Message-ID"] = message_id
        msg.set_content(body)
        with smtplib.SMTP(self.settings.smtp_host, self.settings.smtp_port, timeout=5) as smtp:
            if self.settings.smtp_starttls:
                import ssl

                smtp.starttls(context=ssl.create_default_context())
            if self.settings.smtp_username:
                smtp.login(self.settings.smtp_username, self.settings.smtp_password or "")
            smtp.send_message(msg)
