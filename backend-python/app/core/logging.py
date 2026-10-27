import json
import logging
import time
from uuid import uuid4

from starlette.middleware.base import BaseHTTPMiddleware

logger = logging.getLogger("biletflow.requests")


class RequestLoggingMiddleware(BaseHTTPMiddleware):
    async def dispatch(self, request, call_next):
        request_id = str(uuid4())
        start = time.monotonic()
        try:
            response = await call_next(request)
        except Exception:
            # Exception messages may contain SQL parameters; never log them here.
            logger.error(json.dumps({"requestId": request_id, "status": 500}))
            raise
        response.headers["X-Request-ID"] = request_id
        logger.info(
            json.dumps(
                {
                    "requestId": request_id,
                    "method": request.method,
                    "status": response.status_code,
                    "durationMs": round((time.monotonic() - start) * 1000),
                }
            )
        )
        return response
