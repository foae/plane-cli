# API contract gaps

Known evidence gaps are not implemented API coverage. V1 is best-effort: the supplemental gaps below do not block independently documented P03 operations. Do not invent endpoints or schemas. Available official pages that have not yet been reviewed are unfinished research, not missing upstream contracts.

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
