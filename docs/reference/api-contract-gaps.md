# API contract gaps

Known evidence gaps are not implemented API coverage. V1 is best-effort: the supplemental gaps below do not block independently documented P03 operations. Do not invent endpoints or schemas. Available official pages that have not yet been reviewed are unfinished research, not missing upstream contracts.

## Packet delivery policy

Documented upstream contract gaps are approved exceptions to packet delivery across all packets. Delivery means the independently implementable scope is complete; it does not mean a blocked operation works. Keep blocked source rows distinct from implemented operations and retain planner-review and verification status independently. Available but unread documentation is not an exception.

P03/P04 exceptions are listed below. P06's supplemental public v1 server-mediated uploads remain unimplemented where no authoritative method, path, request and response contract is established. P08's [v1 workspace-member removal](https://developers.plane.so/api-reference/members/remove-workspace-member.md) documents `POST /api/v1/workspaces/{workspace_slug}/members/remove/` but neither its parameters nor its examples identify the member to remove. No guessed destructive payload or callable v1 removal command is exposed. P09 supplemental families require their own evidence reconciliation; the exception policy does not turn unfinished research into coverage.

P07 has no recorded upstream-blocked operation exception. Conflicting examples resolved against explicit operation field declarations remain documented contract decisions, not omitted operations.

| API / operation | Missing evidence | Source / status | Implementation treatment |
|---|---|---|---|
| v1 grouped work-item counts (`work-items/count/`) | Exact method, full path, inputs, response and pagination | [Migration guide](https://developers.plane.so/api-reference/v2/migrating-from-v1.md) names the route only | Evidence-blocked; continue documented operations |
| v1 additional search (`issues/search/`) | Exact operation contract; relationship to indexed searches | [Migration guide](https://developers.plane.so/api-reference/v2/migrating-from-v1.md); no exact contract established | Evidence-blocked; do not alias to documented work-item search |
| v1 `projects-lite/` | Exact method, scope/path, inputs and response | [Migration guide](https://developers.plane.so/api-reference/v2/migrating-from-v1.md) names the route only | Evidence-blocked |
| v1 `members-lite/` | Exact method, scope/path, inputs and response | [Migration guide](https://developers.plane.so/api-reference/v2/migrating-from-v1.md) names the route only | Evidence-blocked |
| v1 `project-members-lite/` | Exact method, scope/path, inputs and response | [Migration guide](https://developers.plane.so/api-reference/v2/migrating-from-v1.md) names the route only | Evidence-blocked |
| v1 `cycles-lite/` | Exact method, scope/path, inputs and response | [Migration guide](https://developers.plane.so/api-reference/v2/migrating-from-v1.md) names the route only | Evidence-blocked |
| v1 `modules-lite/` | Exact method, scope/path, inputs and response | [Migration guide](https://developers.plane.so/api-reference/v2/migrating-from-v1.md) names the route only | Evidence-blocked |
| v1 work-item search | Consistent description of searchable fields | [Search page](https://developers.plane.so/api-reference/issue/search-issues.md): introduction omits description, parameter prose includes it | Pass documented search input through; do not promise exact server matching behavior |

The approved resolutions for conflicting v2 project and v1 work-item write fields are in [API design](../design/api.md). Those conflicts are resolved for local validation and are not blockers. The indexed v1 work-item search and advanced-search operations have their own contracts and are not blocked by the additional search row above.

P04 has an explicit scope exception for the two evidence-blocked v1 `cycles-lite/` and `modules-lite/` routes. They remain unimplemented and do not prevent delivery of P04's independently documented operations. This exception does not establish full public API coverage; authoritative contracts are still required before implementing either route.

The approved v1 inbox migration policy exposes supported `intake` commands only. The [official deprecation notice](https://developers.plane.so/api-reference/inbox-issue/overview.md) ends inbox support on 31 March 2025 and directs callers to replace `/inbox-issues/` with `/intake-issues/`. Inbox source rows are migration aliases, not claims that the old and new wire routes are identical; obsolete inbox routes are deliberately not exposed. Legacy examples also disagree with their parameter prose (`issue` object versus string), so the supported intake contract supplies the request shape.

## P09 supplemental v1 evidence

The [migration guide, section 12](https://developers.plane.so/api-reference/v2/migrating-from-v1.md) identifies AI agents/runs/activities, integration credentials/connections/import jobs, and workflow approvals/state transfers as v1-only without v2 equivalents. It does not establish complete callable contracts. The fragments `work-items/{work_item_id}/workflow-approval/` and `workflows/{workflow_id}/states/{state_id}/transfer/` do not establish methods, full paths, inputs, responses or side effects.

Research on 2026-11-15 checked the [official documentation index](https://developers.plane.so/llms.txt) and targeted public searches of `developers.plane.so/api-reference/` and `github.com/makeplane/plane` for these families. No authoritative operation page or revision-pinned complete public contract was established. Developer-tool agent documentation and OAuth scope names do not establish public REST routes. Search results are not proof that no such API exists.

These three families remain evidence-blocked and unimplemented, under the approved cross-packet upstream-gap delivery policy. They are not counted as implemented coverage. Exact authoritative contracts are required before exposing commands; no methods or schemas are inferred from UI routes or migration fragments. P03/P04/P06 exceptions remain unchanged: the migration guide's mention of v1 `/server/` uploads likewise does not supply complete upload contracts.
