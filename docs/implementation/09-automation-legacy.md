# P09 — Workflows, automations and supplemental v1 coverage

Prerequisites P04/P05/P08. Read [API inventory](../design/api.md#source-inventory-and-coverage), [research limitations](../reference/research.md#findings-requiring-explicit-caution), official migration/type-governance guides and exact operation evidence.

## Source ownership

V2: `project-automations,workspace-automations,workflows,workflow-states,workflow-transitions,workflow-activities,workflow-hooks,work-item-type-workflows,work-item-type-governance,workspace-workflows,workspace-workflow-states,workspace-workflow-transitions,workspace-workflow-activities,workspace-workflow-hooks`.

Supplemental v1 owned here: AI agents/runs/activities, integration credentials/connections/import jobs, workflow approvals/state transfers. P03 owns core specialized searches/grouped counts/lite lists, P04 their scheduling/collaboration equivalents, P06 public server-upload variants; each resolves evidence before closing. P09 reconciles those completed records without duplicating ownership or postponing prerequisite evidence.

## Implement

Research supplemental gaps before coding: follow official docs/OpenAPI and, where necessary, public upstream route/source evidence pinned to a revision. A UI route's existence does not prove supported public API status. Record exact evidence and disagreements. If access cannot establish an advertised public contract, present it to the planner/user; do not invent paths, silently exclude the family or label the full release complete.

Implement workflow graph/state/transition/hook operations and automation controls with exact scope/governance/conflict behavior. Transition/approval/reset/sync actions require their actual side-effect classification. Do not change workspace mode automatically to satisfy a project command. Handle asynchronous run/import results as accepted with explicit get-status, not “completed”.

Integration credential issuance/rotation uses the shared intentional-secret JSON output and opt-in protected-file contract; ordinary reads/errors stay redacted and fixtures use synthetic secrets only. Upstream jobs are not automatically retried, cancelled or rolled back after local interruption unless an explicit documented command requests it.

## Acceptance

Each family includes realistic graph/conflict/permission and side-effect cases; accepted jobs retain safe handles; unknown mutation outcomes remain visible. Verify v1-only commands fail locally under v2, while upstream 404 is not reclassified. Reconcile every supplemental gap with evidence or an explicitly unresolved blocker. Full API scope cannot pass P10 while required gaps remain unresolved without user-approved scope change.
