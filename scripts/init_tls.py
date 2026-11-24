"""Generate a local development CA and localhost certificate without installing trust."""

import subprocess
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
folder = ROOT / ".secrets/tls"
folder.mkdir(parents=True, exist_ok=True, mode=0o700)


def run(*args):
    subprocess.run(
        ["openssl", *args], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL
    )


if not (folder / "ca.key").exists():
    run(
        "req",
        "-x509",
        "-newkey",
        "rsa:3072",
        "-nodes",
        "-days",
        "365",
        "-subj",
        "/CN=BiletFlow local development CA",
        "-addext",
        "basicConstraints=critical,CA:TRUE",
        "-keyout",
        str(folder / "ca.key"),
        "-out",
        str(folder / "ca.crt"),
    )
    (folder / "ca.key").chmod(0o600)
if not (folder / "server.key").exists():
    run(
        "req",
        "-newkey",
        "rsa:3072",
        "-nodes",
        "-subj",
        "/CN=localhost",
        "-keyout",
        str(folder / "server.key"),
        "-out",
        str(folder / "server.csr"),
    )
    (folder / "server.key").chmod(0o600)
    ext = folder / "server.ext"
    ext.write_text(
        "subjectAltName=DNS:localhost,IP:127.0.0.1\nbasicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature,keyEncipherment\nextendedKeyUsage=serverAuth\n"
    )
    run(
        "x509",
        "-req",
        "-in",
        str(folder / "server.csr"),
        "-CA",
        str(folder / "ca.crt"),
        "-CAkey",
        str(folder / "ca.key"),
        "-CAcreateserial",
        "-days",
        "30",
        "-extfile",
        str(ext),
        "-out",
        str(folder / "server.crt"),
    )
for name in ["ca.crt", "server.crt"]:
    (folder / name).chmod(0o644)
print("Local TLS files ready. Trust is not installed into the operating system.")
