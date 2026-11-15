# Workflow governance (v2)

Project picks and workspace policy are separate. All commands require explicit `--api-version v2 --workspace SLUG`. Project commands additionally require `--project ID_OR_IDENTIFIER`; workspace governance and pins never change workspace mode automatically.

| Command | Native operation |
| --- | --- |
| `work-item-type-workflow list` | GET project `work-item-types/workflows/`; bare array, no pagination or sparse fields |
| `work-item-type-workflow get --type UUID` | GET project `work-item-types/{type}/workflow/` |
| `work-item-type-workflow update --type UUID --body-file FILE --yes` | PATCH project pick |
| `work-item-type-governance get --type UUID` | Existing P05 GET workspace governance |
| `work-item-type-governance update --type UUID --body-file FILE --yes` | PATCH workspace policy |
| `work-item-type-governance preview --type UUID --body-file FILE` | Nonmutating POST workspace preview |
| `work-item-type-governance pin list --type UUID` | Paginated GET pins; supports sparse fields and offset/cursor traversal |
| `work-item-type-governance pin get --type UUID --id UUID` | GET pin |
| `work-item-type-governance pin create --type UUID --body-file FILE --yes` | POST pins; native array, newest first |
| `work-item-type-governance pin delete --type UUID --id UUID --yes` | DELETE marker, 204; keeps the project's pick |

Local `--dry-run` never sends a request. Project update's distinct `--server-preview=true` sends PATCH with `dry_run=true` and writes nothing upstream; it runs permission, mandate, pin and allowlist checks. It ignores mappings and reports default target states, not an automatic mapping for the real update. Workspace preview is a read-only POST but still requires workspace workflows write permission.

Server previews use read-only transport and output classification: a failed preview does not report an unknown mutation outcome or a remote change. Real updates still require confirmation and retain mutation-outcome reporting.

## Request bodies and consequences

Project update accepts `workflow_id` (required UUID) and optional `state_mapping`, an object mapping current state IDs to target state UUIDs. Every stranded state must map into the target workflow. Unknown/inactive workflows fail. Required policy refuses project updates with 403 `workflow_mandated`; pins refuse them with 403 `workflow_assigned_by_workspace`. Constrained policy refuses picks outside its allowlist with 400 `workflow_outside_allowlist`.

Workspace preview/update require `mode`: `any`, `constrained`, or `required`. Constrained requires `workflow_ids`; required requires `required_workflow_id`. Update additionally accepts `acknowledge` and `state_mapping`.

- `any` clears the allowlist, keeps pins and changes no picks.
- `constrained` replaces the allowlist. Removing an in-use workflow returns 409 `workflow_allowlist_excludes_in_use` unless `acknowledge:true`. Acknowledgment drops affected picks and pins outside the allowlist; projects fall back to the workspace default workflow. Map stranded states into that default workflow.
- `required` without `acknowledge:true` is an upstream dry run returning native `{ "preview": ... }`. Acknowledgment writes the mandate into every project pick and deletes all pins. Map stranded states into the mandated workflow. `--yes` is local consent, not an implicit `acknowledge` field.

Preview tolerates inactive/unknown constrained IDs by leaving them out; an unknown required workflow previews zero. Update does not have these permissive semantics. Preview counts have one entry per state and affected project; aggregate duplicate state IDs yourself.

Pin creation requires `workflow_id` and nonempty UUID array `project_ids`. Every project must belong to the workspace; otherwise 400 includes `unknown_project_ids` and nothing is pinned. Existing pins move to the new workflow. **Pins accept no mapping:** stranded work items move to the pinned workflow's default state and record activities. Required policy returns 409 `workflow_pins_unavailable`. Preview the destructive impact using `workflow project preview-fallback` before pinning.

The orphan gate returns 409 `workflow_state_orphans` without writes if mappings are incomplete or target states invalid. Non-UUID mapping values return 400; upstream ignores invalid UUID keys. Archived items count and move, drafts do not, triage-group items never move. Moves over 500 items in one project can continue in the background after the response; the CLI does not claim completion, retry or roll back.

Project routes use `projects.workflows:read/write`; workspace routes use `workspaces.workflows:read/write`. Upstream enforces role permissions (403), feature entitlements (402), and governed workspace mode (409 `states_managed_at_project`). Pin reads alone also work in ungoverned workspaces. Responses remain native inside the shared CLI success envelope; errors retain the shared safe local envelope and HTTP status.

Evidence: official API v2 work-item-type-workflows and work-item-type-governance operation pages, recorded in `api/sources/p09-governance-*`.
