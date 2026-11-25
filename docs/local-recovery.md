# Local release and recovery

Use the local HTTPS commands in the [README](../README.md) to start Python,
PostgreSQL, Go, Mailpit and the web interface. Both backends share the database.
Python owns account identity and its Alembic migrations. Go owns orders, tickets,
support, business notifications, sales settings and reporting. HTTPS sends
requests through the same web proxy used by those clients.

## Verify a database backup

The commands below assume the HTTPS Compose project is `biletflow-integrated`.
Use a fresh filename for each backup. Put the running app into a maintenance
window and stop its writers before taking a snapshot:

```sh
docker compose -p biletflow-integrated -f compose.yaml -f deploy/compose.https.yaml stop gateway web go python
python3 scripts/database_recovery.py backup .backups/release.dump
python3 scripts/database_recovery.py rehearse .backups/release.dump
docker compose -p biletflow-integrated -f compose.yaml -f deploy/compose.https.yaml up -d --wait
```

The dump contains both services' tables, migration ledgers and uploaded event
covers. Its companion JSON file contains the archive checksum and table counts
and fingerprints. The rehearsal restores to a randomly named temporary database
and compares its snapshot with the backup. It drops only that temporary database.
Keep the files private and preserve the matching `.env` and `.secrets` separately.
In particular, the original ticket QR signing secret is needed to validate
previously issued tickets.

## Prepare or roll back a release

Record the exact approved commit used for the demo and keep an archive of that
revision. The archive includes tracked files only.

```sh
git archive --format=tar.gz --output=biletflow-source.tar.gz HEAD
```

Before changing versions, keep a verified database backup made with the previous
version. To roll back a schema change, check out the previous commit in a separate
directory and start a fresh Compose project there. Restore the matching backup
into that project's database before starting the older application. Use the same
signing keys and ticket secret, and validate login, an existing ticket and event
history before switching back. Leave the newer project's volume intact.
