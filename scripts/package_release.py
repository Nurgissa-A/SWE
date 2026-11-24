"""Package an exact committed Aliar component; excludes local secrets and private backups."""

import argparse
import hashlib
import json
import subprocess
import tarfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--ref", default="HEAD")
    parser.add_argument("--output", type=Path, default=ROOT / "dist/release")
    args = parser.parse_args()
    sha = subprocess.check_output(["git", "rev-parse", args.ref], cwd=ROOT, text=True).strip()
    args.output.mkdir(parents=True, exist_ok=True)
    archive = args.output / ("aliar-" + sha[:12] + ".tar.gz")
    if archive.exists():
        parser.error("Archive already exists")
    with archive.open("wb") as stream:
        subprocess.run(
            ["git", "archive", "--format=tar.gz", sha], cwd=ROOT, stdout=stream, check=True
        )
    with tarfile.open(archive) as package:
        for item in package.getnames():
            parts = Path(item).parts
            if any(
                part in {".secrets", ".backups", ".storage", ".env", "node_modules", ".venv"}
                for part in parts
            ):
                archive.unlink()
                raise SystemExit("Refusing archive containing local secrets/dependencies")
    images = {}
    for tag in ["biletflow-identity:local", "biletflow-go:latest", "biletflow-web:local"]:
        result = subprocess.run(
            ["docker", "image", "inspect", tag, "--format", "{{.Id}}"],
            capture_output=True,
            text=True,
        )
        if result.returncode:
            raise SystemExit("Build all component images before packaging")
        images[tag] = result.stdout.strip()
        revision = subprocess.check_output(
            [
                "docker",
                "image",
                "inspect",
                images[tag],
                "--format",
                '{{index .Config.Labels "org.opencontainers.image.revision"}}',
            ],
            text=True,
        ).strip()
        if revision != sha:
            archive.unlink()
            raise SystemExit(
                "Image source revision differs from packaged commit; rebuild exact ref"
            )
    manifest = {
        "component": "Aliar Python platform and local deployment",
        "commit": sha,
        "archive": archive.name,
        "sha256": hashlib.sha256(archive.read_bytes()).hexdigest(),
        "images": images,
        "migrationHead": "0005_event_images",
        "scope": "Component release candidate, not accepted whole-team MVP",
        "unresolvedDependencies": [
            "Go ticket/order/payment/campaign/check-in/admin business modules",
            "peer review and final team/instructor acceptance",
        ],
    }
    (args.output / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")
    print("Exact committed source archive and image manifest created without local secrets.")


if __name__ == "__main__":
    main()
