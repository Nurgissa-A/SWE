import json
import logging
import time
from uuid import UUID, uuid4

from starlette.middleware.base import BaseHTTPMiddleware
from starlette.responses import JSONResponse

logger = logging.getLogger("biletflow.requests")


if not logger.handlers:
    logger.addHandler(logging.StreamHandler())
logger.setLevel(logging.INFO)
logger.propagate = False


class RequestLoggingMiddleware(BaseHTTPMiddleware):
    async def dispatch(self, request, call_next):
        candidate = request.headers.get("X-Request-ID", "")
        try:
            request_id = str(UUID(candidate)) if len(candidate) in (32, 36) else str(uuid4())
        except ValueError:
            request_id = str(uuid4())
        request.state.request_id = request_id
        start = time.monotonic()
        try:
            response = await call_next(request)
        except Exception:
            # Exception messages may contain SQL parameters; never log them here.
            logger.error(json.dumps({"requestId": request_id, "status": 500}))
            response = JSONResponse(
                {"error": {"code": "500", "message": "Internal server error"}}, status_code=500
            )
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
