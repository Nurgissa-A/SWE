import os
from pathlib import Path

import pytest
from alembic.config import Config
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import rsa
from fastapi.testclient import TestClient
from sqlalchemy import create_engine, text
from sqlalchemy.engine import make_url

from alembic import command
from app.core.config import Settings
from app.main import create_app


@pytest.fixture(scope="session")
def key_files(tmp_path_factory):
    folder = tmp_path_factory.mktemp("jwt")
    key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    private, public = folder / "private.pem", folder / "public.pem"
    private.write_bytes(
        key.private_bytes(
            serialization.Encoding.PEM,
            serialization.PrivateFormat.PKCS8,
            serialization.NoEncryption(),
        )
    )
    public.write_bytes(
        key.public_key().public_bytes(
            serialization.Encoding.PEM, serialization.PublicFormat.SubjectPublicKeyInfo
        )
    )
    return private, public


@pytest.fixture
def settings(key_files):
    return Settings(jwt_private_key_file=key_files[0], jwt_public_key_file=key_files[1])


@pytest.fixture(scope="session")
def database_url():
    url = os.getenv("TEST_DATABASE_URL")
    if not url:
        pytest.skip("TEST_DATABASE_URL must point to a disposable PostgreSQL *_test database")
    if not make_url(url).database.endswith("_test"):
        pytest.fail("Refusing destructive tests outside a *_test database")
    original = os.environ.get("DATABASE_URL")
    os.environ["DATABASE_URL"] = url
    command.upgrade(Config(str(Path(__file__).parents[1] / "alembic.ini")), "head")
    if original is None:
        os.environ.pop("DATABASE_URL", None)
    else:
        os.environ["DATABASE_URL"] = original
    return url


class MemoryMailer:
    def __init__(self):
        self.messages = []

    def send(self, recipient, purpose, token):
        self.messages.append((recipient, purpose, token))


@pytest.fixture
def ctx(settings, database_url):
    engine = create_engine(database_url)
    with engine.begin() as conn:
        conn.execute(text("TRUNCATE identity_users CASCADE"))
    engine.dispose()
    app = create_app(settings.model_copy(update={"database_url": database_url}))
    mailer = MemoryMailer()
    app.state.mailer = mailer
    with TestClient(app) as client:
        yield client, app, mailer


def registered(ctx, email="person@example.com", verify=True):
    client, _, mailer = ctx
    response = client.post(
        "/api/auth/register", json={"email": email, "password": "safe-password-123"}
    )
    assert response.status_code == 201, response.text
    if verify:
        response = client.post("/api/auth/verify-email", json={"token": mailer.messages[-1][2]})
        assert response.status_code == 200
    return email


def logged_in(ctx, email="person@example.com"):
    registered(ctx, email)
    response = ctx[0].post(
        "/api/auth/login", json={"email": email, "password": "safe-password-123"}
    )
    assert response.status_code == 200, response.text
    return {"Authorization": "Bearer " + response.json()["token"]}
