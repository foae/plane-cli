# API contract gaps

Known evidence gaps are not implemented API coverage. V1 is best-effort: the supplemental gaps below do not block independently documented P03 operations. Do not invent endpoints or schemas. Available official pages that have not yet been reviewed are unfinished research, not missing upstream contracts.

## Packet delivery policy

Documented upstream contract gaps are approved exceptions to packet delivery across all packets. Delivery means the independently implementable scope is complete; it does not mean a blocked operation works. Keep blocked source rows distinct from implemented operations and retain planner-review and verification status independently. Available but unread documentation is not an exception.

P03/P04 exceptions are listed below. P06's content exceptions and supplemental v1 uploads are detailed below. P08's [v1 workspace-member removal](https://developers.plane.so/api-reference/members/remove-workspace-member.md) documents `POST /api/v1/workspaces/{workspace_slug}/members/remove/` but neither its parameters nor its examples identify the member to remove. No guessed destructive payload or callable v1 removal command is exposed. P09's supplemental evidence is reconciled below; the exception policy does not turn unfinished research into coverage.

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

### P06 v1 attachment contract decisions

The reviewed v1 attachment pages are independent contracts, not aliases to v2.
The [list page](https://developers.plane.so/api-reference/issue-attachments/get-attachments.md)
documents a collection GET but illustrates a single JSON object; the CLI preserves
the native response without inventing a pagination envelope.
The [detail page](https://developers.plane.so/api-reference/issue-attachments/get-attachment-detail.md)
explicitly describes a presigned-download redirect, not a metadata response. Its
success status is unspecified; the CLI accepts supported HTTP redirect statuses
without claiming any particular one is the upstream contract.

The [credential page](https://developers.plane.so/api-reference/issue-attachments/get-upload-credentials.md)
documents POST with required `name` and `size`, and optional `type`, `external_id`
and `external_source`. Its success example contains only `detail` and says
"download" despite the upload prose. The CLI exposes that native credential
operation but does not invent a v1 `upload_data`/`asset_id` response envelope.
Use the separate `attachment upload-file --api-version v1 --file PATH
--body-file credentials.json` storage phase with an explicitly supplied
`{"url":"SIGNED_TARGET","fields":{"Content-Type":"MIME","key":"KEY","policy":"POLICY","x-amz-signature":"SIGNATURE"}}`
container. Include every additional returned signed form field. This container is
CLI transport input, not a claim about Plane's credential response shape. The
[storage contract](https://developers.plane.so/api-reference/issue-attachments/upload-file.md)
is multipart POST to the returned target, followed by a separate
`attachment confirm-upload` operation; storage receives no Plane credentials.

The [v1 user asset PATCH](https://developers.plane.so/api-reference/assets/update-user-asset.md)
marks an asset uploaded at `/api/v1/assets/user-assets/{asset_id}/`, accepts optional
`attributes`, and returns empty `204`. `entity_type` appears only in examples and
is not promoted to a required field. This maps independently to
`user-asset confirm-upload --api-version v1`; v2's path and response differ.
These documented operations are implemented, not upstream-gap exceptions.
Only the separately mentioned v1 `/server/` upload variants retain the exception
above because their complete callable contracts remain unspecified.

### P06 content exceptions

These reviewed sources remain unimplemented; none contributes an implemented
operation to coverage. Their recorded source rows retain the precise conflict.

| Sources | Missing or contradictory contract | Treatment |
|---|---|---|
| v2 work-item-page, project-page, workspace-page, sticky and collection lists | Cursor mode is advertised without a cursor continuation parameter | No invented continuation schema or silently offset-only operation |
| v2 project-page, workspace-page, sticky and collection create/update | Writable access/editor metadata declarations disagree with examples or resource types | No guessed coercion, nullability or reduced writable subset |
| v2 collection member management | Nested member/access requirements and types are not pinned by the parameter schema | No invented nested write schema |
| v1 work-item-page detail/delete | `page_id` identity is ambiguous between a page and its link | No v2 identity assumptions, especially for deletion |
| v1 project/workspace page creation | Editor object declarations conflict with string examples; workspace placement references an unlisted `collection_id` | No guessed editor or placement contract |
| v1 sticky create/update | Writable `deleted_at` semantics/nullability do not establish destructive behavior | No guessed destructive classification |
| Supplemental v1 published-page read | Migration prose supplies a route and access differences, but no independent v1 method, inputs, response or error contract | No v1/v2 response equivalence inferred |

The indexed operation pages are linked from their source records in
`api/catalog.json`; independently authored evidence is retained under `api/sources/`.
The supplemental published-page reference is in the
[v2 published-page overview](https://developers.plane.so/api-reference/v2/published-pages/overview.md).
It names `/api/v1/workspaces/{slug}/projects/{project_id}/pages/published/{anchor}/`,
says v1 ignores `project_id` and permits cross-workspace anchor lookup, and supplies
no separate v1 operation link. Those differences prevent treating it as a v2 alias.

### P06 collection native-contract decisions

The reviewed v2 delete and page-membership operations do not need invented
cascade, overlap-precedence, batch-limit or transaction guarantees to execute
their documented requests. Missing guarantees are not whole-operation gaps.
Likewise, the member-list and page-search pages explicitly demonstrate single
JSON objects. Preserve those native responses; do not manufacture a collection
envelope, pagination or a completeness guarantee. Page search documents only
`fields`, not a caller-supplied search string; v1 query parameters do not carry over.


## P09 supplemental v1 evidence

The [migration guide, section 12](https://developers.plane.so/api-reference/v2/migrating-from-v1.md) identifies AI agents/runs/activities, integration credentials/connections/import jobs, and workflow approvals/state transfers as v1-only without v2 equivalents. It does not establish complete callable contracts. The fragments `work-items/{work_item_id}/workflow-approval/` and `workflows/{workflow_id}/states/{state_id}/transfer/` do not establish methods, full paths, inputs, responses or side effects.

Research on 2026-11-15 checked the [official documentation index](https://developers.plane.so/llms.txt) and targeted public searches of `developers.plane.so/api-reference/` and `github.com/makeplane/plane` for these families. No authoritative operation page or revision-pinned complete public contract was established. Developer-tool agent documentation and OAuth scope names do not establish public REST routes. Search results are not proof that no such API exists.

These three families remain evidence-blocked and unimplemented, under the approved cross-packet upstream-gap delivery policy. They are not counted as implemented coverage. Exact authoritative contracts are required before exposing commands; no methods or schemas are inferred from UI routes or migration fragments. P03/P04/P06 exceptions remain unchanged: the migration guide's mention of v1 `/server/` uploads likewise does not supply complete upload contracts.
