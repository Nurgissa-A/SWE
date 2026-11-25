from contextlib import asynccontextmanager

import httpx
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from starlette.middleware.trustedhost import TrustedHostMiddleware

from app.api.routes import covers, health, identity
from app.core.body_limit import BodyLimitMiddleware
from app.core.config import Settings
from app.core.errors import install_error_handlers
from app.core.logging import RequestLoggingMiddleware
from app.core.security import TokenService
from app.db.session import make_engine, make_session_factory
from app.services.email import SMTPEmailSender


def create_app(settings: Settings | None = None) -> FastAPI:
    settings = settings or Settings()

    @asynccontextmanager
    async def lifespan(app: FastAPI):
        app.state.tokens = TokenService(settings)
        with httpx.Client(base_url=settings.events_api_url, timeout=2, trust_env=False) as events:
            app.state.events = events
            try:
                yield
            finally:
                app.state.engine.dispose()

    app = FastAPI(title="BiletFlow Identity API", version="0.1.0", lifespan=lifespan)
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
        allow_headers=["Authorization", "Content-Type"],
    )
    app.add_middleware(RequestLoggingMiddleware)
    install_error_handlers(app)
    app.include_router(health.router)
    app.include_router(identity.router)
    app.include_router(covers.router)

    @app.get("/.well-known/jwks.json", tags=["identity"])
    def jwks():
        return app.state.tokens.jwks()

    return app


app = create_app()
