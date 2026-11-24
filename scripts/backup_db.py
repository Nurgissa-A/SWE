"""Create a private PostgreSQL custom-format backup using the local Compose service."""

import argparse
import hashlib
import json
import os
import subprocess
from pathlib import Path

from backup_common import snapshot


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    root = Path(__file__).resolve().parents[1]
    path = args.output.resolve()
    if path.exists():
        parser.error("Refusing to overwrite an existing backup")
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    before = snapshot("biletflow")
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    try:
        with os.fdopen(fd, "wb") as stream:
            subprocess.run(
                [
                    "docker",
                    "compose",
                    "exec",
                    "-T",
                    "postgres",
                    "pg_dump",
                    "-U",
                    "biletflow",
                    "-Fc",
                    "--no-owner",
                    "biletflow",
                ],
                cwd=root,
                stdout=stream,
                check=True,
            )
    except BaseException:
        path.unlink(missing_ok=True)
        raise
    after = snapshot("biletflow")
    if before != after:
        raise SystemExit("Backup created, but source changed; repeat in a quiet maintenance window")
    manifest = path.with_suffix(path.suffix + ".json")
    with os.fdopen(os.open(manifest, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600), "w") as stream:
        json.dump(
            {"sha256": hashlib.sha256(path.read_bytes()).hexdigest(), "snapshot": before},
            stream,
            sort_keys=True,
        )
    print("Private PostgreSQL backup created. Keep encryption/signing keys separately.")


if __name__ == "__main__":
    main()
