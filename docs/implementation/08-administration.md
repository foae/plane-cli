# P08 — Workspace/project administration and integrations

Prerequisites P03/P05. Read [authentication](../design/authentication.md), [CLI safety](../design/cli-contract.md#safety-and-dry-run), [API coverage](../design/api.md).

## Source ownership

V1: `members,workspace-invitations,idp-group-sync,project-features,workspace-features`, plus user administration from `user` (identity reads belong to P03). V2: `members,invitations,roles,permissions,permission-grants,permission-schemes,group-sync,project-features,workspace-features,audit-logs,notifications,artifacts,webhooks,webhook-logs`, plus user administration from `users`. P03 owns all project lifecycle and state/label definition/assignment operations; P09 owns workflow states. Do not split one operation between packets based on whether its HTTP verb appears administrative.

## Implement

Review the operation ledger first so each shared-family action has one owner. Explicit project/workspace scope and exact role IDs; no client-side inferred permission model. Security/role/grant changes and member removal require --yes, even for POST/PATCH. Feature toggles are explicit commands, never preconditions hidden inside another write.

Implement invitations, group sync and documented administrative reads/writes/actions. Distinguish metadata from sync execution. Webhook create/rotate intentionally returns declared one-time secret fields in JSON by default; advertise optional `--secret-output-file` and apply its preflight/partial-delivery contract. Ordinary reads and diagnostics remain redacted. Deletion/revocation do not silently cascade beyond documented API behavior.

## Acceptance

Fixtures cover insufficient role, missing versus hidden entity, feature entitlement, scope correctness, secret-returning creation/rotation, destructive classification before any request and local credential isolation. Pagination covers logs/audits without unbounded --all. No real invites/webhooks/permission changes are sent. Add permission/admin recipes with side effects explicit and safe placeholders.

## Delivery evidence

The default review panel (`20261115-122721-351669`) completed five seats; scoring was disabled. Accepted repairs cover mutation uncertainty for the side-effecting v1 group-sync GET, post-redaction aggregate byte limits, nested signing credentials in protected-file receipts, opaque/string-valued log headers, missing signing secrets, and bare-email validation.

Intentional one-time secret stdout is the approved delivery contract, not a disclosure defect. V2-only handlers are version-gated before execution; required typed schemas protect bulk invitation assertions. The conservative command annotation and v1 runtime mutation classification are deliberate. Ordinary audit/invitation content remains native, not globally scrubbed personal data; HTTP failures expose safe local errors rather than upstream bodies. Dry-run does not prepare protected files. Additional single-page/post-install failure tests were suggestions, not demonstrated failures.

The isolated dry-run privacy finding remains: request URLs can contain supplied sensitive query/path values. Treat dry-run output as sensitive; metadata-driven URL redaction was not included in the qualifying repair set.

Verification recorded `go test ./internal/cli -run '^TestP08' -count=1`, `go test ./internal/httpx -count=1`, and `make check` passing. Initial regression expectations incorrectly retained opaque headers and omitted a base URL; those fixtures were corrected. Fake-server acceptance does not establish live edition compatibility. The v1 removal exception is recorded in [API contract gaps](../reference/api-contract-gaps.md); it is not an implemented command.
