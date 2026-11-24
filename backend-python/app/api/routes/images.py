import hashlib
import io
import os
import warnings
from datetime import UTC, datetime
from uuid import UUID, uuid4

from fastapi import APIRouter, Body, Depends, HTTPException, Request
from fastapi.responses import FileResponse
from PIL import Image, ImageOps, UnidentifiedImageError
from sqlalchemy import text
from sqlalchemy.orm import Session

from app.api.dependencies import require_roles
from app.db.session import get_session
from app.models.identity import Role, User
from app.models.platform import AdminAudit, EventImage
from app.schemas.identity import APIModel

router = APIRouter(tags=["platform"])
Image.MAX_IMAGE_PIXELS = 8_000_000


class ImageResponse(APIModel):
    event_id: UUID
    image_url: str
    sha256: str
    width: int
    height: int
    updated_at: datetime


def output(row):
    return {
        "eventId": row.event_id,
        "imageUrl": "/api/event-images/" + str(row.event_id),
        "sha256": row.sha256,
        "width": row.width,
        "height": row.height,
        "updatedAt": row.updated_at,
    }


@router.put("/api/event-images/{event_id}", response_model=ImageResponse)
def put_image(
    event_id: UUID,
    request: Request,
    body: bytes = Body(media_type="application/octet-stream"),
    user: User = Depends(require_roles(Role.ORGANIZER, for_update=True)),
    session: Session = Depends(get_session),
):
    content_type = request.headers.get("content-type", "").split(";")[0].lower()
    if content_type not in {"image/png", "image/jpeg", "image/webp"}:
        raise HTTPException(415, "Only PNG, JPEG or static WebP images are accepted")
    request.app.state.business.staff(
        user, event_id, request.headers["Authorization"], request.state.request_id
    )
    if len(body) > 1048576:
        raise HTTPException(413, "Image exceeds 1 MiB")
    try:
        with warnings.catch_warnings():
            warnings.simplefilter("error", Image.DecompressionBombWarning)
            with Image.open(io.BytesIO(body), formats=["PNG", "JPEG", "WEBP"]) as source:
                expected = {"PNG": "image/png", "JPEG": "image/jpeg", "WEBP": "image/webp"}[
                    source.format
                ]
                if expected != content_type or getattr(source, "n_frames", 1) != 1:
                    raise HTTPException(422, "MIME mismatch or animated image")
                if (
                    source.width > 4096
                    or source.height > 4096
                    or source.width * source.height > 8_000_000
                ):
                    raise HTTPException(422, "Image dimensions exceed the supported limit")
                source.load()
                clean = ImageOps.exif_transpose(source).convert("RGB")
                data = io.BytesIO()
                clean.save(data, format="WEBP", quality=85, exif=b"", icc_profile=b"", xmp=b"")
                encoded = data.getvalue()
                width, height = clean.size
    except (
        UnidentifiedImageError,
        Image.DecompressionBombError,
        Image.DecompressionBombWarning,
        OSError,
        ValueError,
    ):
        raise HTTPException(422, "Invalid or unsafe image") from None
    if len(encoded) > 1048576:
        raise HTTPException(413, "Normalized image exceeds 1 MiB")
    session.execute(text("SELECT pg_advisory_xact_lock(64721,3)"))
    folder = request.app.state.settings.image_storage_dir
    folder.mkdir(parents=True, exist_ok=True, mode=0o700)
    digest = hashlib.sha256(encoded).hexdigest()
    path = folder / (digest + ".webp")
    if not path.exists():
        if (
            sum(item.stat().st_size for item in folder.glob("*.webp")) + len(encoded)
            > 50 * 1024 * 1024
        ):
            raise HTTPException(413, "Local image storage quota reached")
        temporary = folder / ("." + str(uuid4()) + ".tmp")
        try:
            with os.fdopen(
                os.open(temporary, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600), "wb"
            ) as stream:
                stream.write(encoded)
                stream.flush()
                os.fsync(stream.fileno())
            os.replace(temporary, path)
        finally:
            temporary.unlink(missing_ok=True)
    row = session.get(EventImage, event_id)
    if row and row.sha256 == digest:
        return output(row)
    if row is None:
        row = EventImage(event_id=event_id, created_by=user.id)
        session.add(row)
    row.sha256, row.width, row.height, row.updated_at = digest, width, height, datetime.now(UTC)
    session.add(
        AdminAudit(
            actor_id=user.id,
            action="event_image",
            target_id=str(event_id),
            detail="Normalized WebP " + digest,
        )
    )
    session.commit()
    return output(row)


@router.get("/api/event-images/{event_id}", response_class=FileResponse)
def get_image(event_id: UUID, request: Request, session: Session = Depends(get_session)):
    request.app.state.business.context(
        actor_id=None,
        role="viewer",
        token=request.headers.get("Authorization", ""),
        request_id=request.state.request_id,
        event_id=event_id,
    )
    row = session.get(EventImage, event_id)
    if not row:
        raise HTTPException(404, "Event image not found")
    folder = request.app.state.settings.image_storage_dir.resolve()
    path = folder / (row.sha256 + ".webp")
    if path.is_symlink() or not path.is_file() or path.resolve().parent != folder:
        raise HTTPException(404, "Event image not found")
    return FileResponse(
        path,
        media_type="image/webp",
        headers={"X-Content-Type-Options": "nosniff", "Cache-Control": "private, no-store"},
    )
