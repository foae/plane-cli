# Initiatives and teamspaces

These commands implement the documented v1/v2 initiative, initiative-label and
teamspace operations. Protocol acceptance uses deterministic fake servers, not a
live Plane deployment; entitlement and edition support remain upstream concerns.

```sh manual
plane-cli initiative list --api-version v2 --workspace my-team --all
plane-cli initiative get --api-version v2 --workspace my-team --id INITIATIVE_ID --output table
plane-cli initiative-label list --api-version v2 --workspace my-team
plane-cli teamspace list --api-version v2 --workspace my-team --search Engineering
plane-cli schema initiative create --api-version v2
```

Create an initiative with a JSON file containing `{"name":"Launch"}`:

```sh manual
plane-cli initiative create --api-version v2 --workspace my-team --body-file initiative.json
```

V1 exposes separate add/remove operations for initiative projects, labels and
epics, and teamspace projects/members. V2 initiative associations use explicit
`manage-projects`, `manage-labels` and `manage-work-items` commands with native
`add`/`remove` arrays. Nonempty removals require `--yes`; additions alone do not.
V2 initiative updates containing `project_ids`, and teamspace updates containing
`member_ids` or `project_ids`, replace the current set and require `--yes` even
when the array is empty. Metadata-only updates do not require confirmation.
This classification uses field presence without fetching existing membership.
Deletion requires `--yes` in both versions. Use `--dry-run` to inspect scope and
confirmation requirements without authentication or HTTP requests.

Do not normalize version-specific payloads. For example, v2 initiative
`logo_props` is documented as a string; fields with explicit nullable contracts
accept JSON null. V1 teamspace membership uses its documented native payload,
not the v2 representation. Unsupported version/action combinations fail locally;
there is no fallback to another API generation.

Lists preserve the selected generation's cursor/offset pagination. `--all`
aggregates pages within the command deadline. V2 sparse `--fields` and `--expand`
selectors are forwarded only where documented. List/get tables project `id` and
`name`; sparse responses can leave cells empty.

Related families have separate guides: [customers](customers.md),
[Plane releases](releases-api.md), and [templates/views](templates-views.md).
