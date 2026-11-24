# Aliar delivery evidence

Local verification date: 2026-11-24. Scope is the Aliar component through Week 12;
see [the task matrix](aliar-weeks-1-12.md) for external dependencies and human
acceptance. Changes are submitted for review.

## Executed checks

| Check | Actual result |
|---|---|
| Python/PostgreSQL full suite | 97 passed; one existing Starlette/httpx test-client deprecation warning |
| CI-mode local full suite | Same 97 passed with CI=true, verbose pytest and faulthandler |
| Go fmt/vet/test | Passed; 16 top-level tests, 32 including subtests; GitHub also executes race detector |
| Spectral/shared OpenAPI/generated TypeScript | Passed; zero Spectral warnings, no generated drift |
| Web typecheck/build | Passed locally/in Docker; GitHub web job additionally verifies runtime audit |
| Frontend runtime npm audit | Zero vulnerabilities reported |
| Real runtime PostgreSQL ACL | Identity/notifier/events isolation passed; password-hash denial verified |
| Real HTTPS/SMTP/Go smoke | Passed; verification email, profile, support/idempotent reply/notifications, admin settings/suspension/audit, Event ownership and image upload/restart persistence, logout enforced by Go |
| Clean installation | New biletflow-clean-rc Compose project and PostgreSQL volume; all migrations/bootstrap/grants and real smoke passed; containers/network then removed |
| Database restore | Real schema-0005 custom-format dump restored to a NEW biletflow_aliar_0005_restore_test; all 12 tracked table counts/fingerprints and migration revision matched; disposable target removed |
| Image restore | Archive read/restored into isolated temporary directory, every SHA256 and DB reference verified |
| Pinned rollback and return | Distinct OCI image sets ab70953 → e8e7c10, then rollback e8e7c10 → ab70953 and return → e8e7c10; HTTPS smoke passed after both switches; retained schema was never downgraded |
| Release guard failures | Incompatible schema and false source provenance rejected before deployment; mismatched source/image packaging rejected and incomplete archive removed |

Private recovery files remain in ignored `.backups/`, including
`aliar-0005.dump`, its JSON proof and `aliar-0005-images.tar.gz` plus proof.
Local intermediate exact-source manifests are in ignored `dist/rc-ab70953/` and
`dist/rc-e8e7c10/`. They contain no live keys or database records. Final package and
CI status are linked from the delivery PR; the manifest identifies the exact
source SHA and immutable image IDs. Keep images/private key backups separately.

## Review and handover

Review order: identity stability (`feature/identity-stability`) → Python platform
(`feature/python-platform`) → local release (`release/aliar-platform`).
Approval and merge are separate from implementation.

Use [the release runbook](local-release.md), [API behavior](python-platform.md),
[architecture](architecture.md), [ERD](erd.md), [JWT guide](python-go-auth.md) and
[OpenAPI](openapi.yaml). Documentation and scripts are included in the source
archive. For local launch after installing locked dependencies:

```sh
python scripts/local_release.py --build
python scripts/smoke_platform.py
```

Open https://localhost:8443/login or `/docs`; use `.secrets/tls/ca.crt` for explicit
local CA verification. CA trust is not installed automatically.

Madi still supplies persistent Go Events, orders/checkout/tickets/PDF, campaigns,
staff/admission and business moderation/financial producers. Web owners consume
support/admin/ticket APIs. Contract fixtures are not a real purchase/check-in.
Independent teammate recovery rehearsal and final team/instructor demo must be
performed externally; executable demo/recovery instructions are supplied.
