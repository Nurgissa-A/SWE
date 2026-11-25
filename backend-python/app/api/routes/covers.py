from io import BytesIO
from typing import Annotated
from uuid import UUID

import httpx
from fastapi import APIRouter, Body, Depends, HTTPException, Path, Request, Response
from PIL import Image, ImageOps
from sqlalchemy.dialects.postgresql import insert
from sqlalchemy.orm import Session

from app.api.dependencies import require_roles
from app.db.session import get_session
from app.models.identity import EventCover, Role, User
from app.schemas.identity import APIModel

router = APIRouter(prefix="/api/event-covers", tags=["identity"])
MAX_BYTES = 1024 * 1024
EventID = Annotated[UUID, Path(alias="eventId")]


class CoverResponse(APIModel):
    image_url: str


def event(request: Request, event_id: UUID, authorization: str | None = None) -> dict:
    headers = {"Authorization": authorization} if authorization else {}
    try:
        response = request.app.state.events.get(f"/api/events/{event_id}", headers=headers)
        if response.status_code in (401, 403, 404):
            raise HTTPException(404, "Event not found")
        response.raise_for_status()
        data = response.json()
        if not isinstance(data, dict) or data.get("id") != str(event_id):
            raise ValueError("Unexpected event response")
        return data
    except (httpx.HTTPError, ValueError):
        raise HTTPException(503, "Event service unavailable") from None


def webp(data: bytes, content_type: str) -> bytes:
    formats = {"image/jpeg": "JPEG", "image/png": "PNG", "image/webp": "WEBP"}
    image_format = formats.get(content_type)
    if image_format is None:
        raise HTTPException(415, "Choose a JPEG, PNG or WebP image")
    try:
        with Image.open(BytesIO(data), formats=[image_format]) as image:
            if image.width * image.height > 8_000_000 or getattr(image, "n_frames", 1) != 1:
                raise HTTPException(422, "Choose a static image of at most 8 megapixels")
            clean = ImageOps.exif_transpose(image).convert("RGB")
            clean.thumbnail((1600, 1600))
            clean.info.clear()
            output = BytesIO()
            clean.save(output, format="WEBP", quality=82)
    except (OSError, ValueError, SyntaxError, Image.DecompressionBombError):
        raise HTTPException(422, "Invalid image") from None
    if output.tell() > MAX_BYTES:
        raise HTTPException(413, "Image is too large")
    return output.getvalue()


@router.put("/{eventId}", response_model=CoverResponse)
def upload_cover(
    event_id: EventID,
    request: Request,
    data: Annotated[bytes, Body(media_type="application/octet-stream")],
    owner: User = Depends(require_roles(Role.ORGANIZER)),
    session: Session = Depends(get_session),
):
    details = event(request, event_id, request.headers.get("Authorization"))
    if details.get("organizerId") != str(owner.id):
        raise HTTPException(403, "Only the event organizer can change its cover")
    if details.get("visibility") == "private":
        raise HTTPException(422, "Uploaded covers are available for public and unlisted events")
    normalized = webp(data, request.headers.get("content-type", "").split(";", 1)[0].lower())
    statement = insert(EventCover).values(event_id=event_id, body=normalized)
    session.execute(
        statement.on_conflict_do_update(
            index_elements=[EventCover.event_id], set_={"body": normalized}
        )
    )
    session.commit()
    return {"imageUrl": f"/api/event-covers/{event_id}"}


@router.get("/{eventId}", response_class=Response)
def get_cover(event_id: EventID, request: Request, session: Session = Depends(get_session)):
    cover = session.get(EventCover, event_id)
    if cover is None:
        raise HTTPException(404, "Cover not found")
    # The caller is an HTML img. Ask Go for anonymous visibility before serving any bytes.
    event(request, event_id)
    return Response(
        cover.body,
        media_type="image/webp",
        headers={"Cache-Control": "no-store", "X-Content-Type-Options": "nosniff"},
    )
