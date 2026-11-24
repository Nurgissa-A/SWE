"""Restore only into a new disposable *_restore_test DB and verify row counts/versions."""

import argparse
import hashlib
import json
import re
import subprocess
from pathlib import Path

from backup_common import ROOT, execute, snapshot


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--backup", type=Path, required=True)
    parser.add_argument("--target", required=True)
    args = parser.parse_args()
    if not re.fullmatch(r"biletflow_[a-z0-9_]+_restore_test", args.target):
        parser.error("Target must be a new biletflow_*_restore_test database")
    proof = json.loads(args.backup.with_suffix(args.backup.suffix + ".json").read_text())

    if hashlib.sha256(args.backup.read_bytes()).hexdigest() != proof["sha256"]:
        parser.error("Backup checksum mismatch")
    # createdb fails for an existing database: never silently replace existing data.
    execute("createdb", "-U", "biletflow", args.target)
    try:
        with args.backup.open("rb") as stream:
            subprocess.run(
                [
                    "docker",
                    "compose",
                    "exec",
                    "-T",
                    "postgres",
                    "pg_restore",
                    "-U",
                    "biletflow",
                    "-d",
                    args.target,
                    "--no-owner",
                    "--exit-on-error",
                ],
                cwd=ROOT,
                stdin=stream,
                capture_output=True,
                check=True,
            )
        restored = snapshot(args.target)
        assert restored["revision"] == "0005_event_images", "Unexpected migration head"
        assert restored == proof["snapshot"], "Restored rows differ from the stable backup snapshot"
        print(
            json.dumps(
                {"result": "restored", "database": args.target, "tables": restored}, sort_keys=True
            )
        )
    finally:
        # Only the DB this command created is removed, including on failed restore.
        execute("dropdb", "-U", "biletflow", "--force", args.target)


if __name__ == "__main__":
    main()
