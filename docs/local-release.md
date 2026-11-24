# Aliar local HTTPS release and recovery

This is the Aliar component release candidate. The Go order/ticket/check-in,
financial/campaign and moderation implementations and independent team acceptance
remain external dependencies. Use Docker Desktop/Engine with Compose 2.24+, Git,
OpenSSL, Python 3.12 and the locked Python dependencies. Node 24 is needed only for
host contract/web checks; application builds run inside Docker.

## Clean installation

Check out the release tag or recorded commit, create a Python environment and install
`backend-python/requirements.lock` plus `pip install --no-deps --no-build-isolation -e backend-python`.
From the repository root, run:

```sh
python scripts/local_release.py --build
curl --cacert .secrets/tls/ca.crt https://localhost:8443/ready
curl --cacert .secrets/tls/ca.crt https://localhost:8443/health/go
python scripts/smoke_platform.py
```

The release builder refuses uncommitted source and labels images with the source
commit. Local initialization preserves existing keys/passwords; no live credentials
are in Git. The first build downloads pinned base images and locked dependencies.
On Linux, run initialization as the account that owns the checkout; generated
LOCAL_UID/GID allow the API/gateway to read private mounted files without root.

Open https://localhost:8443/login, `/docs` or `/redoc`. The local development CA is
`.secrets/tls/ca.crt`; a browser will require a local trust exception. No system trust
is installed automatically. Curl/smoke verify this CA and never disable TLS checks.
Only port 8443 is published by this overlay. Mailpit, DB, worker and internal API
stay on the Compose network. Go remains authoritative for Event visibility.
Dev-only Prism is a separate tools profile; stop it with `docker compose stop prism`
when presenting only the release gateway.

TLS leaf lifetime is 30 days and CA lifetime 365 days. To renew the leaf, stop gateway,
move server.key/server.crt/server.csr/server.ext out of `.secrets/tls` into private
operator storage, run `python scripts/init_tls.py`, then restart gateway. Retain the CA
key/certificate pair; do not regenerate it casually. Inspect expiry with
`openssl x509 -in .secrets/tls/server.crt -noout -dates`.

## State, migrations and access

PostgreSQL data lives in the named `postgres-data` volume; image blobs live in
`.storage/event-images`. Keep `.env` (database role passwords), JWT key pair,
notification.key and service-token in separate protected backup storage. Losing
notification.key prevents decryption of queued messages. Losing JWT private material
requires deliberate session/key rotation and coordinated Go configuration.

Startup order is PostgreSQL → runtime-role bootstrap → Alembic migrations → runtime
privilege grants → worker/API → Go/web → gateway. Python migration head is
`0005_event_images`, with upgrades `0001_identity` → `0002_platform` →
`0003_sessions` → `0004_operator_audit` → `0005_event_images`. The migration owner
has DDL privileges; identity, notifier and events runtime accounts do not. Python
never migrates Go-owned business tables. Future Go persistence requires Madi's
migration service and read-only runtime migration ledger permissions.

```sh
docker compose run --rm -T --no-deps db-permissions python -m app.db.roles check
docker compose -f compose.yaml -f deploy/compose.https.yaml ps
docker compose logs --tail 50 python notification-worker
```

Operator role/disable/enable commands are in the Python README. Never publish their
credentials or mailbox contents. Audit records retain operator/account actions.
SMTP outbox is durable with bounded retry and at-least-once transport semantics;
private notification status/retry APIs are Platform Admin only. Five failed attempts
require operator investigation/retry. Restart worker to recover expired leases;
never create a replacement order to resend a notification.

## Backup and restore rehearsal

Use a quiet maintenance window. Stop public writes and worker before recording a
consistent DB/assets pair; leave postgres and one-shot admin tools available:

```sh
docker compose -f compose.yaml -f deploy/compose.https.yaml stop gateway notification-worker
python scripts/backup_db.py --output .backups/release.dump
python scripts/backup_assets.py --output .backups/release-images.tar.gz
python scripts/rehearse_restore.py --backup .backups/release.dump --target biletflow_release_restore_test
docker compose -f compose.yaml -f deploy/compose.https.yaml start notification-worker gateway
```

Backups and their checksums/fingerprints are private mode-0600 artifacts excluded
from Git and release archives. The rehearsal creates a NEW disposable database,
checks the migration head and all tracked identity/support/notification/settings/
audit/image-pointer row fingerprints, and removes only that disposable database.
An existing target is never overwritten. Asset backup verifies content-addressed
file hashes and every DB image reference, then actually reads/restores the archive
into an isolated temporary directory. Preserve its JSON proof with the DB proof.

For actual recovery, keep the original DB/volume intact, create a new database with
`createdb`, restore with `pg_restore --no-owner --exit-on-error`, verify the same
snapshot procedure, bootstrap/grant runtime roles, and point a separate checkout's
connection URLs at the recovered database. Restore image members to a NEW directory:
accept only regular files named `[0-9a-f]{64}.webp`, verify each SHA256 against its
name, then mount that directory at `/srv/storage/event-images`. Preserve the matching
notification/JWT/service keys. Verify `/ready`, role checks and smoke in the isolated
checkout before switching the gateway. Do not use `down -v`, destructive downgrade,
`pg_restore --clean`, or overwrite the retained database as a recovery shortcut.

## Pinned release and rollback

After successful checks, build the committed revision, then package it:

```sh
python scripts/package_release.py --ref HEAD --output dist/release
python scripts/local_release.py --manifest dist/release/manifest.json
```

The source tar.gz excludes secrets/private state. Its manifest records source SHA,
archive digest, immutable local Docker image IDs, schema compatibility and unresolved
team dependencies. Packaging checks OCI source labels against the commit. Preserve
these images locally or export them to protected offline storage with `docker image save`;
a source archive/manifest alone does not contain image bytes or private data.

Keep each prior manifest and its images. Roll back by passing that prior manifest;
the launcher validates image provenance and matching retained schema BEFORE replacing
containers. It never downgrades a database. Current manifests support only
`0005_event_images`; an earlier Week 1–4 image is unsafe for retained session/support
state and is rejected. For a different schema, rehearse a separate restore and use
an explicitly compatible application version. After rollback rerun CA-verified
health/smoke and inspect delivery/audit counts. Restore the newer manifest to roll
forward. Peer review/merge and a second team member's recovery exercise are separate
acceptance gates, never implied by the operator's local rehearsal.

## Demonstration and handover

Run `smoke_platform.py` for real verification email/login, organizer profile,
Platform Admin suspension/settings/audit, support creation/reply/idempotent retries,
Go Event ownership, image upload and restart persistence, session logout enforced
by Go, and CA verification. It creates disposable demo users/events and uses the
private demo mailbox via Compose; run only on a local demonstration installation.
Show the source manifest, passing CI, API documentation, restore proof and the
Weeks 1–12 matrix. Orders/ticket PDF, paid activation, campaigns and admission are
pending Go/client acceptance; their contract fixtures are not a real purchase demo.
