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
env = root / ".env"
if not env.exists():
    with os.fdopen(os.open(env, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600), "w") as stream:
        stream.write("POSTGRES_PASSWORD=" + secrets.token_hex(24) + "\n")
        # Windows has no getuid/getgid; Docker Desktop does not need the host IDs there.
        uid = getattr(os, "getuid", lambda: 10001)()
        gid = getattr(os, "getgid", lambda: 10001)()
        stream.write(f"LOCAL_UID={uid}\nLOCAL_GID={gid}\n")
# Keys added after the first release are appended to an existing .env without touching other values.
existing = env.read_text()
if "TICKET_QR_SECRET=" not in existing:
    with env.open("a") as stream:
        stream.write("TICKET_QR_SECRET=" + secrets.token_hex(32) + "\n")
print("Local configuration ready; no existing values overwritten.")
