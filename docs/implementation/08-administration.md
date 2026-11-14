# P08 — Workspace/project administration and integrations

Prerequisites P03/P05. Read [authentication](../design/authentication.md), [CLI safety](../design/cli-contract.md#safety-and-dry-run), [API coverage](../design/api.md).

## Source ownership

V1: `members,workspace-invitations,idp-group-sync,project-features,workspace-features,project-labels`, plus administrative actions explicitly split from P03 `user,project,state,label`. V2: `members,invitations,roles,permissions,permission-grants,permission-schemes,group-sync,project-features,workspace-features,workspace-states,states,labels,audit-logs,notifications,artifacts,webhooks,webhook-logs`, plus administrative actions split from `users,projects,project-states,project-labels`.

## Implement

Review the operation ledger first so each shared-family action has one owner. Explicit project/workspace scope and exact role IDs; no client-side inferred permission model. Security/role/grant changes and member removal require --yes, even for POST/PATCH. Feature toggles are explicit commands, never preconditions hidden inside another write.

Implement invitations, group sync and documented administrative reads/writes/actions. Distinguish metadata from sync execution. Webhook create/rotate may expose secret material: normal output is redacted; intentional delivery uses the protected secret-output contract. Do not log auth headers or webhook secrets in debug/errors. Deletion/revocation do not silently cascade beyond documented API behavior.

## Acceptance

Fixtures cover insufficient role, missing versus hidden entity, feature entitlement, scope correctness, secret-returning creation/rotation, destructive classification before any request and local credential isolation. Pagination covers logs/audits without unbounded --all. No real invites/webhooks/permission changes are sent. Add permission/admin recipes with side effects explicit and safe placeholders.
