"""Create ignored local credentials/keys; never overwrite existing configuration."""

import os
import secrets
import subprocess
from pathlib import Path

root = Path(__file__).resolve().parents[1]
keys = root / ".secrets"
keys.mkdir(mode=0o700, exist_ok=True)
private = keys / "jwt-private.pem"
public = keys / "jwt-public.pem"
if not private.exists():
    subprocess.run(
        [
            "openssl",
            "genpkey",
            "-algorithm",
            "RSA",
            "-pkeyopt",
            "rsa_keygen_bits:3072",
            "-out",
            str(private),
        ],
        check=True,
        stderr=subprocess.DEVNULL,
    )
    private.chmod(0o600)
if not public.exists():
    subprocess.run(
        ["openssl", "pkey", "-in", str(private), "-pubout", "-out", str(public)],
        check=True,
    )
    public.chmod(0o644)
for name, content in [
    ("notification.key", __import__("base64").urlsafe_b64encode(secrets.token_bytes(32))),
    ("service-token", secrets.token_urlsafe(48).encode()),
]:
    path = keys / name
    if not path.exists():
        with os.fdopen(os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600), "wb") as stream:
            stream.write(content)
env = root / ".env"
if not env.exists():
    with os.fdopen(os.open(env, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600), "w") as stream:
        stream.write("POSTGRES_PASSWORD=" + secrets.token_hex(24) + "\n")
        stream.write(f"LOCAL_UID={os.getuid()}\nLOCAL_GID={os.getgid()}\n")
existing = env.read_text()
missing = []
for name in [
    "IDENTITY_DATABASE_PASSWORD",
    "NOTIFIER_DATABASE_PASSWORD",
    "EVENTS_DATABASE_PASSWORD",
]:
    if not any(line.startswith(name + "=") for line in existing.splitlines()):
        missing.append(name + "=" + secrets.token_hex(24) + "\n")
if missing:
    with env.open("a") as stream:
        if existing and not existing.endswith("\n"):
            stream.write("\n")
        stream.writelines(missing)
env.chmod(0o600)
print("Local configuration ready; no existing files overwritten.")

(root / ".storage/event-images").mkdir(parents=True, exist_ok=True, mode=0o700)
