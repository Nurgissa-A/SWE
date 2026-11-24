"""Back up immutable Event images and rehearse restore into a new temporary directory."""

import argparse
import hashlib
import io
import json
import os
import re
import tarfile
import tempfile
from pathlib import Path

from backup_common import ROOT, execute


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    path = args.output.resolve()
    if path.exists():
        parser.error("Refusing to overwrite an existing image archive")
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    folder = ROOT / ".storage/event-images"
    entries = {}
    for file in sorted(folder.glob("*.webp")):
        if file.is_symlink() or not re.fullmatch(r"[0-9a-f]{64}\.webp", file.name):
            parser.error("Unexpected image file")
        data = file.read_bytes()
        if hashlib.sha256(data).hexdigest() != file.stem:
            parser.error("Image digest mismatch")
        entries[file.name] = data
    references = (
        execute(
            "psql",
            "-U",
            "biletflow",
            "-d",
            "biletflow",
            "-Atc",
            "SELECT sha256 FROM platform_event_images",
        )
        .decode()
        .splitlines()
    )
    if any(value + ".webp" not in entries for value in references):
        parser.error("Database references a missing image; stop uploads and repeat")
    with os.fdopen(os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600), "wb") as stream:
        with tarfile.open(fileobj=stream, mode="w:gz") as archive:
            for name, data in entries.items():
                info = tarfile.TarInfo(name)
                info.size, info.mode = len(data), 0o600
                archive.addfile(info, io.BytesIO(data))
    # Read the real archive and restore bytes only into our isolated directory.
    with tempfile.TemporaryDirectory(prefix="biletflow-images-restore-") as temporary:
        with tarfile.open(path) as archive:
            restored = {}
            for member in archive:
                if not member.isfile() or member.name not in entries:
                    raise SystemExit("Unsafe archive member")
                data = archive.extractfile(member).read()
                destination = Path(temporary) / member.name
                destination.write_bytes(data)
                restored[member.name] = hashlib.sha256(destination.read_bytes()).hexdigest()
        assert restored == {name: name.removesuffix(".webp") for name in entries}
    proof = {
        "result": "image restore rehearsed",
        "files": len(entries),
        "databaseReferences": len(references),
        "sha256": hashlib.sha256(path.read_bytes()).hexdigest(),
    }
    with os.fdopen(
        os.open(str(path) + ".json", os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600), "w"
    ) as stream:
        json.dump(proof, stream, sort_keys=True)
    print(json.dumps(proof, sort_keys=True))


if __name__ == "__main__":
    main()
