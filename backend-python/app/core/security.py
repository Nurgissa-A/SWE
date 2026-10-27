import hashlib
import secrets
from datetime import UTC, datetime, timedelta
from uuid import UUID, uuid4

import jwt
from argon2 import PasswordHasher
from argon2.exceptions import InvalidHashError, VerificationError
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import rsa
from jwt.algorithms import RSAAlgorithm

from app.core.config import Settings
from app.models.identity import User

password_hasher = PasswordHasher(time_cost=3, memory_cost=65536, parallelism=2)
DUMMY_HASH = password_hasher.hash(secrets.token_urlsafe(32))


def hash_password(password: str) -> str:
    return password_hasher.hash(password)


def verify_password(hashed: str, password: str) -> bool:
    try:
        return password_hasher.verify(hashed, password)
    except (VerificationError, InvalidHashError):
        return False


def opaque_token() -> tuple[str, str]:
    raw = secrets.token_urlsafe(32)
    return raw, token_digest(raw)


def token_digest(raw: str) -> str:
    return hashlib.sha256(raw.encode()).hexdigest()


class TokenService:
    def __init__(self, settings: Settings):
        self.settings = settings
        self.private_key = serialization.load_pem_private_key(
            settings.jwt_private_key_file.read_bytes(), password=None
        )
        self.public_key = serialization.load_pem_public_key(
            settings.jwt_public_key_file.read_bytes()
        )
        if not isinstance(self.private_key, rsa.RSAPrivateKey) or self.private_key.key_size < 2048:
            raise ValueError("RS256 signing requires an RSA key of at least 2048 bits")
        if not isinstance(self.public_key, rsa.RSAPublicKey):
            raise ValueError("RS256 verification requires an RSA public key")
        if self.private_key.public_key().public_numbers() != self.public_key.public_numbers():
            raise ValueError("JWT key pair does not match")

    def issue(self, user: User) -> str:
        now = datetime.now(UTC)
        return jwt.encode(
            {
                "sub": str(user.id),
                "role": user.role,
                "ver": user.token_version,
                "iss": self.settings.jwt_issuer,
                "aud": self.settings.jwt_audience,
                "iat": now,
                "nbf": now,
                "exp": now + timedelta(minutes=self.settings.access_token_minutes),
                "jti": str(uuid4()),
            },
            self.private_key,
            algorithm="RS256",
            headers={"kid": self.settings.jwt_key_id},
        )

    def decode(self, token: str) -> dict:
        header = jwt.get_unverified_header(token)
        if header.get("kid") != self.settings.jwt_key_id:
            raise jwt.InvalidTokenError("Unknown key")
        claims = jwt.decode(
            token,
            self.public_key,
            algorithms=["RS256"],
            issuer=self.settings.jwt_issuer,
            audience=self.settings.jwt_audience,
            options={"require": ["sub", "role", "ver", "iss", "aud", "iat", "nbf", "exp", "jti"]},
        )
        UUID(claims["sub"])
        if type(claims["ver"]) is not int or claims["ver"] < 0:
            raise jwt.InvalidTokenError("Invalid token version")
        return claims

    def jwks(self) -> dict:
        import json

        key = json.loads(RSAAlgorithm.to_jwk(self.public_key))
        return {"keys": [{**key, "kid": self.settings.jwt_key_id, "alg": "RS256", "use": "sig"}]}
