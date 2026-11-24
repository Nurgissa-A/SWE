import hashlib
from contextlib import asynccontextmanager

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from starlette.middleware.trustedhost import TrustedHostMiddleware

from app.api.routes import health, identity, images, platform
from app.core.body_limit import BodyLimitMiddleware
from app.core.config import Settings
from app.core.errors import install_error_handlers
from app.core.logging import RequestLoggingMiddleware
from app.core.security import TokenService
from app.db.session import make_engine, make_session_factory
from app.services.business import BusinessClient
from app.services.email import SMTPEmailSender
from app.services.notifications import Notifications


def create_app(settings: Settings | None = None) -> FastAPI:
    settings = settings or Settings()

    @asynccontextmanager
    async def lifespan(app: FastAPI):
        app.state.tokens = TokenService(settings)
        app.state.abuse_key = hashlib.sha256(settings.notification_key_file.read_bytes()).digest()
        app.state.notifications = Notifications(
            settings.notification_key_file.read_bytes(), settings.frontend_url
        )
        app.state.service_token = settings.service_token_file.read_text().strip()
        if len(app.state.service_token) < 32:
            raise ValueError("Internal service token must have at least 32 characters")
        app.state.business = BusinessClient(settings.business_url)
        try:
            yield
        finally:
            app.state.business.close()
            app.state.engine.dispose()

    app = FastAPI(title="BiletFlow Identity and Platform API", version="0.3.0", lifespan=lifespan)
    app.state.settings = settings
    app.state.mailer = SMTPEmailSender(settings)
    app.state.engine = make_engine(settings.database_url, settings)
    app.state.session_factory = make_session_factory(app.state.engine)
    app.add_middleware(BodyLimitMiddleware)
    app.add_middleware(TrustedHostMiddleware, allowed_hosts=settings.trusted_hosts)
    app.add_middleware(
        CORSMiddleware,
        allow_origins=settings.cors_origins,
        allow_methods=["GET", "POST", "PATCH", "PUT"],
        allow_headers=["Authorization", "Content-Type", "Idempotency-Key"],
        expose_headers=["X-Request-ID"],
    )
    app.add_middleware(RequestLoggingMiddleware)
    install_error_handlers(app)
    app.include_router(health.router)
    app.include_router(identity.router)
    app.include_router(platform.router)
    app.include_router(images.router)

    @app.get("/.well-known/jwks.json", tags=["identity"])
    def jwks():
        return app.state.tokens.jwks()

    return app


app = create_app()
