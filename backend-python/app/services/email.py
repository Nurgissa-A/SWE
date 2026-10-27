import smtplib
from email.message import EmailMessage
from typing import Protocol
from urllib.parse import urlencode

from app.core.config import Settings


class EmailSender(Protocol):
    def send(self, recipient: str, purpose: str, token: str) -> None: ...


class SMTPEmailSender:
    def __init__(self, settings: Settings):
        self.settings = settings

    def send(self, recipient: str, purpose: str, token: str) -> None:
        page = "verify-email" if purpose == "verify_email" else "reset-password"
        link = f"{self.settings.frontend_url.rstrip('/')}/{page}?{urlencode({'token': token})}"
        msg = EmailMessage()
        msg["From"] = self.settings.email_from
        msg["To"] = recipient
        msg["Subject"] = f"BiletFlow {page.replace('-', ' ')}"
        msg.set_content(f"Open this single-use link: {link}\nIf you did not request it, ignore it.")
        with smtplib.SMTP(self.settings.smtp_host, self.settings.smtp_port, timeout=5) as smtp:
            if self.settings.smtp_starttls:
                import ssl

                smtp.starttls(context=ssl.create_default_context())
            if self.settings.smtp_username:
                smtp.login(self.settings.smtp_username, self.settings.smtp_password or "")
            smtp.send_message(msg)
