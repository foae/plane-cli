# Templates and saved views (v2)

These commands preserve the documented REST payloads. They do not translate MCP PQL, infer a scope, resolve names, probe entitlements, or retry writes. Use `--api-version v2` explicitly or select a v2 connection.

| Resource | Scope | Actions |
| --- | --- | --- |
| `view` | required `--scope project\|workspace` | `list`, `get`, `create`, `update`, `delete` |
| `work-item-template` | required `--scope project\|workspace` | `list`, `get`, `create`, `update`, `delete`; `use` only with project scope |
| `page-template` | required `--scope project\|workspace` | `list`, `get`, `create`, `update`, `delete` |
| `project-template` | workspace-owned, no scope flag | `list`, `get`, `create`, `update`, `delete`, `copy`, `use` |
| `template-category` | global, no workspace flag | `list`, `get` only |

Supply `--workspace` for every resource except global categories. Project scope also requires `--project`; these v2 project routes accept a project UUID or identifier. The target resource uses `--id`. Workspace and project page templates are disjoint; a missing or governed project template is never retried through workspace routes.

An explicit `--project` conflicts with `--scope workspace` and is rejected before any request. A configured default project does not change an explicitly selected workspace scope.

## Read and filter

```sh
plane-cli view list --api-version v2 --workspace team --scope project --project ENG --access 0 --is-locked=false --count=false
plane-cli page-template get --api-version v2 --workspace team --scope workspace --id TEMPLATE_ID --fields all
plane-cli template-category list --api-version v2 --is-active=false --output table
plane-cli project-template list --api-version v2 --workspace team --category-id-in CATEGORY_A,CATEGORY_B --is-published=true --is-verified=true
```

Lists preserve native envelopes. `--all`, `--max-pages`, `--per-page`, `--offset`, `--paginate cursor`, and `--cursor` use native v2 continuation. Template lists support publication, short-id and search filters; project/page template lists also support exact case-insensitive name filters. View lists support integer access, lock, name, owner-id and search filters. Filters combine upstream, without conversion to another query language. `--fields` is passed unchanged; `--expand` appears only where the operation documents it. Lists and detail reads support meaningful `--output table` projections.

Project/page template ordering is `created_at` or `id`, optionally prefixed by `-`. Category ordering additionally supports `sort_order` and `name`, but cursor mode requires `--order-by created_at` or `id` (optionally descending), not the default `sort_order`. `--is-active=false` selects only inactive categories, including on `get`; it does not mean both active and inactive.

## Saved views

Create and update accept a full native JSON object with `--body-file FILE` (or `-` for stdin). Saved `filters`, `display_filters`, `display_properties`, `logo_props`, and `pql_filters` are documented **strings**. The CLI neither builds their editor-specific contents nor translates PQL. The stored `query` field is read-only. Write `access` is documented as a string even though response access and list access filters are integers; the operation-specific write contract wins over the overview/example discrepancy.

Updating `access` changes visibility and requires `--yes`, including when the caller sends the current value. Metadata-only view updates do not require confirmation.

```sh
printf '%s\n' '{"name":"High priority","access":"0","filters":"{\"priority\":[\"high\"]}","display_filters":"{\"layout\":\"board\"}","is_locked":false}' |
  plane-cli view create --api-version v2 --workspace team --scope project --project ENG --body-file -
```

## Create, replace and apply templates

Template create bodies require `name` and `template_data`. The template's `description_html` describes the template; `template_data.description_html` is the seeded work item/page body. Preserve HTML and editor-owned descriptors unchanged. Page seeds require a name; a nonempty parent requires `id`, `project_id` (version-4 UUIDs), and `project_identifier`. Shape checking does not prove a referenced page exists.

```sh
printf '%s\n' '{"name":"Weekly notes","template_data":{"name":"Weekly sync","description_html":"<h2>Agenda</h2>","view_props":{"full_width":false}}}' |
  plane-cli page-template create --api-version v2 --workspace team --scope workspace --body-file -
plane-cli work-item-template use --api-version v2 --workspace team --scope project --project ENG --id TEMPLATE_ID --body-file overrides.json
plane-cli project-template copy --api-version v2 --workspace destination --body-file copy.json
plane-cli project-template use --api-version v2 --workspace destination --id LOCAL_TEMPLATE_ID --body-file new-project.json
```

`overrides.json` can be `{}` or contain `name`. An optional body `project_id` is accepted upstream but ignored: the work item is always created in the path project. Only template name, HTML and priority apply; the server supplies the new work item's default state and does not apply other saved seed values. The response is the new work item, not the template.

`copy.json` contains `{"template_id":"PUBLISHED_SOURCE_UUID"}`. The source may be in another workspace, but must be published. Copy returns the new destination-owned template with `201`. The server resets publication, verification, categories, source-workspace member/state references and bundled work items; the template description resets too. External cover URLs are reused; a Plane-hosted cover absent from the destination can cause `404` with no copy created.

`new-project.json` requires `name` and `identifier`. This endpoint takes integer `network` (`0` or `2`), and UUID body keys `project_lead` and `default_assignee`, **not** the project-create endpoint's `_id` spellings. Dates may be date strings or ISO date-times, including null. The response is the newly created project with `201`; background labels/types/modules/work items can remain incomplete after that response. The CLI does not poll or claim those steps completed. Template toggles override corresponding request toggles; other settings such as visibility, dates and timezone come from the request. Copy does not apply a project, and use does not create a template.

Updates without `template_data` leave the seed untouched. Sending `template_data` requires `--yes` because omitted content can be removed:

- Page and work-item seeds replace the whole seed; send every key you want retained. Their seed name is required.
- Project seeds require a name and replace all bundled `workitems`; omitting `workitems` removes the old list. Omitted `members` resets to `[]`; omitted/null `cover_asset` resets to empty. Other project seed keys merge upstream.
- The CLI does not pre-read or silently merge the old seed.

All deletes require `--yes`. Ordinary top-level metadata updates do not, except saved-view `access` changes. Use `--dry-run` to inspect method, URL and confirmation classification without credentials or HTTP.

## Native failures

Feature-gated endpoints return native `402 payment_required`; this does not establish a universal edition compatibility rule. Project/page template single-detail reads have documented gating exceptions. Governed project page-template writes return `409 templates_managed_at_workspace`; migrations can return `409 governance_migration_in_progress`. No automatic route switch or feature mutation follows either response. Project seed shape failures may carry an `errors` object rather than the common field-error array.

Contracts and deterministic request fixtures cover all 40 documented operations in the eight assigned families. Their lifecycle is `implemented`, not planner-approved `verified`.
