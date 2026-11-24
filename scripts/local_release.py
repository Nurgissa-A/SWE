"""Start or pin/rollback the local HTTPS image set without dropping PostgreSQL data."""

import argparse
import json
import os
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--manifest", type=Path)
    parser.add_argument("--build", action="store_true")
    args = parser.parse_args()
    if args.manifest and args.build:
        parser.error("A pinned rollback never rebuilds images")
    subprocess.run([sys.executable, "scripts/init_local.py"], cwd=ROOT, check=True)
    subprocess.run([sys.executable, "scripts/init_tls.py"], cwd=ROOT, check=True)
    env = os.environ.copy()
    if args.build:
        if subprocess.check_output(["git", "status", "--porcelain"], cwd=ROOT):
            parser.error("Commit source before building a traceable release")
        env["RELEASE_COMMIT"] = subprocess.check_output(
            ["git", "rev-parse", "HEAD"], cwd=ROOT, text=True
        ).strip()
    if args.manifest:
        manifest = json.loads(args.manifest.read_text())
        # Current release supports only this schema; fail before any container mutation.
        if manifest["migrationHead"] != "0005_event_images":
            parser.error(
                "This image set is not declared compatible with the current migration head"
            )
        current = subprocess.run(
            [
                "docker",
                "compose",
                "exec",
                "-T",
                "postgres",
                "psql",
                "-U",
                "biletflow",
                "-d",
                "biletflow",
                "-Atc",
                "SELECT version_num FROM alembic_version",
            ],
            cwd=ROOT,
            capture_output=True,
            text=True,
            check=True,
        ).stdout.strip()
        if current != manifest["migrationHead"]:
            parser.error("Retained database and image-set schema differ; follow restore runbook")
        mapping = {
            "biletflow-identity:local": "IDENTITY_IMAGE",
            "biletflow-go:latest": "EVENTS_IMAGE",
            "biletflow-web:local": "WEB_IMAGE",
        }
        for tag, name in mapping.items():
            image = manifest["images"][tag]
            subprocess.run(
                ["docker", "image", "inspect", image], check=True, stdout=subprocess.DEVNULL
            )
            revision = subprocess.check_output(
                [
                    "docker",
                    "image",
                    "inspect",
                    image,
                    "--format",
                    '{{index .Config.Labels "org.opencontainers.image.revision"}}',
                ],
                text=True,
            ).strip()
            if revision != manifest["commit"]:
                parser.error("Manifest/image provenance mismatch")
            env[name] = image
        # Forward-compatible image rollback only. Never downgrade a retained database.
        print("Pinning existing images; schema remains at its current forward migration head.")
    cmd = [
        "docker",
        "compose",
        "-f",
        "compose.yaml",
        "-f",
        "deploy/compose.https.yaml",
        "up",
        "-d",
        "--wait",
        "--wait-timeout",
        "180",
    ]
    cmd += ["--build"] if args.build else ["--no-build", "--pull", "never"]
    subprocess.run(cmd, cwd=ROOT, env=env, check=True)
    print("Local HTTPS: https://localhost:8443; verify TLS with .secrets/tls/ca.crt.")


if __name__ == "__main__":
    main()
