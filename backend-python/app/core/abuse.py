"""Persisted fixed-window request budgets; stores keyed digests, never email/IP."""

import hashlib
import hmac
import math
import time
from datetime import UTC, datetime

from fastapi import HTTPException, Request
from sqlalchemy.dialects.postgresql import insert

from app.models.platform import RateLimit


async def auth_budget(request: Request):
    settings = request.app.state.settings
    if not settings.auth_rate_limit_enabled:
        return
    try:
        body = await request.json()
        email = body.get("email", "") if isinstance(body, dict) else ""
        email = email.strip().lower() if isinstance(email, str) else ""
    except ValueError:
        email = ""
    now = time.time()
    window = 900
    bucket = int(now) // window
    until = (bucket + 1) * window
    budgets = [
        (
            "ip",
            request.client.host if request.client else "unknown",
            settings.auth_rate_limit_per_ip,
        )
    ]
    if email:
        budgets.append(("email", email, settings.auth_rate_limit_per_email))
    exhausted = False
    with request.app.state.engine.begin() as connection:
        for kind, value, maximum in budgets:
            key = hmac.new(
                request.app.state.abuse_key,
                f"{request.url.path}:{kind}:{bucket}:{value}".encode(),
                hashlib.sha256,
            ).hexdigest()
            stmt = insert(RateLimit).values(
                key=key, count=1, expires_at=datetime.fromtimestamp(until, UTC)
            )
            count = connection.scalar(
                stmt.on_conflict_do_update(
                    index_elements=[RateLimit.key], set_={"count": RateLimit.count + 1}
                ).returning(RateLimit.count)
            )
            exhausted |= count > maximum
    if exhausted:
        raise HTTPException(
            429,
            "Too many authentication requests; retry later",
            headers={"Retry-After": str(max(1, math.ceil(until - now)))},
        )
