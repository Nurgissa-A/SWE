"""Create a private backup or rehearse its restore in an isolated PostgreSQL database."""

import argparse
import hashlib
import json
import os
import subprocess
from pathlib import Path
from uuid import uuid4

ROOT = Path(__file__).resolve().parents[1]
DATABASE = "biletflow"
DATABASE_ROLE = "biletflow"
SNAPSHOT = r"""
SET TIME ZONE 'UTC';
SELECT format(
    'SELECT json_build_array(%L, %L, count(*),
        coalesce(sum(hashtextextended(to_jsonb(t)::text, 0)), 0)) FROM %I.%I AS t;',
    schemaname, tablename, schemaname, tablename
)
FROM pg_tables
WHERE schemaname NOT IN ('pg_catalog', 'information_schema')
ORDER BY schemaname, tablename
\gexec
"""


def postgres(project, *command, **kwargs):
    return subprocess.run(
        [
            "docker",
            "compose",
            "-p",
            project,
            "exec",
            "-T",
            "postgres",
            *command,
        ],
        cwd=ROOT,
        check=True,
        **kwargs,
    )


def snapshot(project, database):
    output = postgres(
        project,
        "psql",
        "-XqAt",
        "-v",
        "ON_ERROR_STOP=1",
        "-U",
        DATABASE_ROLE,
        "-d",
        database,
        input=SNAPSHOT,
        capture_output=True,
        text=True,
    ).stdout
    return [json.loads(line) for line in output.splitlines()]


def digest(path):
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def backup(project, archive, proof):
    if archive.exists() or proof.exists():
        raise SystemExit("Choose a new backup path")
    archive.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    before = snapshot(project, DATABASE)
    if not {row[1] for row in before} >= {"alembic_version", "go_schema_migrations"}:
        raise SystemExit("Start the full product and run migrations before backing it up")
    descriptor = os.open(archive, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600)
    try:
        with os.fdopen(descriptor, "wb") as stream:
            postgres(
                project,
                "pg_dump",
                "-U",
                DATABASE_ROLE,
                "-Fc",
                "--no-owner",
                DATABASE,
                stdout=stream,
            )
        if before != snapshot(project, DATABASE):
            raise SystemExit("Database changed during backup. Pause writers and retry.")
        manifest = {"sha256": digest(archive), "tables": before}
        with os.fdopen(os.open(proof, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600), "w") as stream:
            json.dump(manifest, stream)
    except BaseException:
        archive.unlink()
        raise
    print(f"Private backup created with {len(before)} tables")


def rehearse(project, archive, proof):
    expected = json.loads(proof.read_text())
    if digest(archive) != expected["sha256"]:
        raise SystemExit("Backup checksum mismatch")
    target = "biletflow_restore_" + uuid4().hex
    postgres(project, "createdb", "-U", DATABASE_ROLE, target)
    try:
        with archive.open("rb") as stream:
            postgres(
                project,
                "pg_restore",
                "-U",
                DATABASE_ROLE,
                "--no-owner",
                "--exit-on-error",
                "-d",
                target,
                stdin=stream,
            )
        restored = snapshot(project, target)
        if restored != expected["tables"]:
            raise SystemExit("Restored data differs from the backup snapshot")
        print(f"Verified {len(restored)} restored tables in an isolated database")
    finally:
        postgres(project, "dropdb", "-U", DATABASE_ROLE, target)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=["backup", "rehearse"])
    parser.add_argument("archive", type=Path)
    parser.add_argument("--project", default="biletflow-integrated")
    args = parser.parse_args()
    archive = args.archive.resolve()
    proof = archive.with_suffix(archive.suffix + ".json")
    operation = backup if args.action == "backup" else rehearse
    operation(args.project, archive, proof)


if __name__ == "__main__":
    main()
