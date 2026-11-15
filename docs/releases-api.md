# Release API workflows

The `release` group implements the documented v1 and v2 release APIs. Workspace releases use `release <action>`; project releases use `release project <action> --project <id>`. Both have `list`, `get`, `create`, `update`, and `delete`. Workspace and project lists are never combined, and a release cannot move between these scopes. Common release list/get commands support `--output table` with ID, name, status, target date, and release date.

## Related resources

- `release tag list|get|create|update|delete` manages the version-tag catalog.
- `release label-catalog list|get|create|update|delete` manages the label palette, not labels attached to a release.
- `release label list` reads a release's attached labels; `release work-item list` reads its work items.
- `release comment list|get|create|update|delete` and `release link list|get|create|update|delete` manage the discussion and external links.
- `release changelog get|update` manages release notes independently of the release description.

In v2, each attached-resource command also exists below `release project`, including labels, work items, changelog, comments, and links. The project route requires `--project`. In v1, project release children use the workspace-style child commands with the project release's `--release` ID; only project release CRUD uses `release project`.

All mutations accept `--body-file PATH` (or `--body-file -` for stdin). Scalar field flags are also available; a body file and body-field flags are mutually exclusive. Use the body file for arrays, editor JSON, null, and object metadata. Version strings are opaque: the tag's `--tag-version` is not semantic-version validated or rewritten. Dates are sent as `YYYY-MM-DD`; no timestamp conversion is performed.

```sh
plane-cli release list --workspace my-team --api-version v2 --output table
plane-cli release project get --workspace my-team --project ENG --release "$RELEASE_ID" --api-version v2
plane-cli release tag create --workspace my-team --api-version v2 --tag-version 'release/2026.11+build-007'
plane-cli release changelog update --workspace my-team --release "$RELEASE_ID" --api-version v2 --body-file notes.json
plane-cli release project work-item manage --workspace my-team --project ENG --release "$RELEASE_ID" --api-version v2 --body-file membership.json --yes
```

For the changelog example, `notes.json` can contain `{"description_html":"<p>Shipped.</p>","description_json":{}}`. HTML and editor JSON are independent upstream fields: omitted fields remain unchanged, and the CLI derives neither from the other. An empty HTML string is supported. **The first changelog GET creates an empty changelog upstream**, so discovery classifies it as a write-side-effect operation despite its GET method.

## Membership and version differences

V2 `label manage` and `work-item manage` send exactly `{"add":["id"],"remove":["id"]}` in a single POST. Removal runs before addition. Native `added` and `removed` arrays are preserved, including skipped IDs and IDs already attached. A nonempty `remove` requires `--yes`; an add-only request does not.

V1 instead exposes `label add|remove` with `label_ids`, and `work-item add|remove` with `work_item_ids`. Removal uses DELETE with its JSON body and requires `--yes`. Every resource deletion also requires `--yes`; `--dry-run` reports confirmation requirements without making requests.

V1 release bodies use `lead` and `tag`; v2 uses `lead_id` and `tag_id`. V1 comment replies use `parent`; v2 uses `parent_id`. Native fields and response objects are not translated between versions.

V1 catalogs can create project-scoped labels/tags with the `project` body field (`--project-id`) and list them with the `project_id` query (`--project-id`). V2 catalogs create/list workspace-level entries only. V2 attachment operations can nevertheless accept project-scoped entries in the same workspace, even from another project. V1 project release membership enforces the documented same-project constraints.

The official workspace v2 link contract declares `metadata` as a **string**; the project v2 link contract and v1 contract declare an **object**. These operation-specific contracts remain distinct; the CLI does not coerce metadata.

Lists use native pagination: one page by default, `--all` to follow continuation, and `--max-pages` to bound traversal. V2 main/catalog/comment/link lists support the documented offset/cursor options. V2 attached-label and work-item lists support offset pagination only, without invented filters or ordering. V1 lists use native cursors. Sparse `--fields` and `--expand` are available only where documented.

## Availability and evidence

Workspace and project releases have separate paid-plan entitlements. The workspace Releases setting additionally gates workspace release creation, not project release creation. The CLI does not preflight settings or fall back to a different scope or API version. Upstream 402, authorization, and scope errors remain failures.

This implementation covers 90 documented operations (38 v1 and 52 v2), with deterministic fake-server operation fixtures. It is implemented, not planner-verified or a claim about a live deployment. All 99 assigned source URLs were fetched and reviewed, including all nine overviews; the overviews introduced no additional operation routes beyond their dedicated operation pages. Independently authored minimal contracts and reader-derived byte hashes are retained under `api/sources/p07-releases-*`; the official URLs below are the durable upstream references.

Official contracts: [v1 releases](https://developers.plane.so/api-reference/release/overview.md), [v2 releases](https://developers.plane.so/api-reference/v2/releases/overview.md), [project releases](https://developers.plane.so/api-reference/v2/project-releases/overview.md), [tags](https://developers.plane.so/api-reference/v2/release-tags/overview.md), [label palette](https://developers.plane.so/api-reference/v2/release-labels/overview.md), [workspace comments](https://developers.plane.so/api-reference/v2/release-comments/overview.md), [workspace links](https://developers.plane.so/api-reference/v2/release-links/overview.md), [project comments](https://developers.plane.so/api-reference/v2/project-release-comments/overview.md), and [project links](https://developers.plane.so/api-reference/v2/project-release-links/overview.md).
