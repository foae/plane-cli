# P08 — Workspace/project administration and integrations

Prerequisites P03/P05. Read [authentication](../design/authentication.md), [CLI safety](../design/cli-contract.md#safety-and-dry-run), [API coverage](../design/api.md).

## Source ownership

V1: `members,workspace-invitations,idp-group-sync,project-features,workspace-features`, plus user administration from `user` (identity reads belong to P03). V2: `members,invitations,roles,permissions,permission-grants,permission-schemes,group-sync,project-features,workspace-features,audit-logs,notifications,artifacts,webhooks,webhook-logs`, plus user administration from `users`. P03 owns all project lifecycle and state/label definition/assignment operations; P09 owns workflow states. Do not split one operation between packets based on whether its HTTP verb appears administrative.

## Implement

Review the operation ledger first so each shared-family action has one owner. Explicit project/workspace scope and exact role IDs; no client-side inferred permission model. Security/role/grant changes and member removal require --yes, even for POST/PATCH. Feature toggles are explicit commands, never preconditions hidden inside another write.

Implement invitations, group sync and documented administrative reads/writes/actions. Distinguish metadata from sync execution. Webhook create/rotate intentionally returns declared one-time secret fields in JSON by default; advertise optional `--secret-output-file` and apply its preflight/partial-delivery contract. Ordinary reads and diagnostics remain redacted. Deletion/revocation do not silently cascade beyond documented API behavior.

## Acceptance

Fixtures cover insufficient role, missing versus hidden entity, feature entitlement, scope correctness, secret-returning creation/rotation, destructive classification before any request and local credential isolation. Pagination covers logs/audits without unbounded --all. No real invites/webhooks/permission changes are sent. Add permission/admin recipes with side effects explicit and safe placeholders.
