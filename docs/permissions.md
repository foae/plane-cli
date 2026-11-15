# Permissions and feature administration

Permission reads, roles, schemes and grants use **v2**. Project/workspace feature get and update support explicit **v1** and **v2**, with different native field names. Commands never discover roles, infer access from numeric role levels, switch API versions, or toggle a feature as a hidden prerequisite.

## Read the caller's access

```sh
plane-cli role list --api-version v2 --workspace TEAM --namespace project
plane-cli role get --api-version v2 --workspace TEAM --id ROLE_UUID --expand permission_schemes
plane-cli permission-scheme list --api-version v2 --workspace TEAM
plane-cli permission-scheme get --api-version v2 --workspace TEAM --id SCHEME_UUID
plane-cli permission workspace-effective --api-version v2 --workspace TEAM
plane-cli permission project-effective --api-version v2 --workspace TEAM --project ENG
plane-cli permission check --api-version v2 --workspace TEAM --resource-type workitem --resource-id WORK_ITEM_UUID --action edit
plane-cli permission resource --api-version v2 --workspace TEAM --resource-type workitem --resource-id WORK_ITEM_UUID
plane-cli permission accessible --api-version v2 --workspace TEAM --resource-type project --action view
plane-cli permission relation --api-version v2 --workspace TEAM --resource-type project --resource-id PROJECT_UUID
```

These are GETs and need no confirmation. Roles and permission schemes are read-only APIs: no create/update/delete commands are invented. UUIDs, role slugs, conditional permissions such as `workitem:delete+creator`, and native response fields remain unchanged.

Use `permission check` to test an action. `permission resource` lists the actions a resource type supports, **not** the actions you are authorized to perform. `relation: null` does not mean no access: inherited and teamspace access are not direct relations. `accessible` omits access inherited only from a parent. All answers concern the caller; there is no on-behalf-of flag. An unknown project on the effective-project route can return 200 with a null relation; the CLI does not reinterpret that as an existence check.

Role/scheme reads accept `--fields`; role reads also accept `--expand permission_schemes`. Role list filters include `--namespace`, `--is-system=false`, `--slug` and `--search`. Grant lists require both `--resource-type` and `--resource-id`; there is no workspace-wide grant listing.

### Pagination

Role, scheme and grant lists preserve native page envelopes. Use documented `--offset`, `--per-page`, `--count=false`, `--order-by` and `--paginate cursor`. The shared official v2 pagination contract supplies `--cursor` continuation even though these leaf parameter tables omit it. Pass the returned opaque `next_cursor` unchanged and retain filters, ordering and page size. Invalid cursor/order combinations remain native server errors.

For bounded aggregation use `--all --max-pages 100` (default bound: 1000). The result retains each native page, rather than flattening resource rows. Effective, check, resource, accessible and relation reads are not paginated and do not accept sparse/expand flags.

## Change grants explicitly

```sh
plane-cli permission-grant list --api-version v2 --workspace TEAM --resource-type project --resource-id PROJECT_UUID
plane-cli permission-grant get --api-version v2 --workspace TEAM --id GRANT_UUID
plane-cli permission-grant create --api-version v2 --workspace TEAM --body-file grant.json --dry-run
plane-cli permission-grant create --api-version v2 --workspace TEAM --body-file grant.json --yes
plane-cli permission-grant update --api-version v2 --workspace TEAM --id GRANT_UUID --body-file patch.json --yes
plane-cli permission-grant delete --api-version v2 --workspace TEAM --id GRANT_UUID --yes
```

`grant.json` uses the exact role slug returned by the roles API; replace the illustrative UUIDs with the intended subject and resource:

```json
{
  "subject_type": "user",
  "subject_id": "26753bc5-5317-4476-b5dc-412f082b1a5e",
  "relation": "guest",
  "resource_type": "project",
  "resource_id": "82a503f0-cb9c-4460-a838-6116c4d51dfe",
  "permissions_grant": ["workitem:create"],
  "permissions_deny": [],
  "expires_at": null
}
```

Every grant mutation requires `--yes` before credential lookup or HTTP, including create: creating the same subject/resource grant **replaces** its values while still returning 201. Omitted permission lists become empty and omitted expiry becomes null on create. PATCH preserves omitted fields; sent lists replace whole lists, `[]` clears a list, and `expires_at: null` makes a grant permanent. Subject/resource identity cannot be changed by PATCH. Use JSON for arrays and explicit nulls; scalar field flags are alternatives to `--body-file`, not additions to it.

Grant APIs require the resource's native manage permission. Grant deletion does not remove member records, nor does it remove access inherited through another grant, parent or teamspace. A successful DELETE returns empty 204 (`data: null`), with no `--fields` option. Reads/create/update support documented sparse fields. Missing/hidden/expired entities and insufficient access remain native 404/403 responses, not inferred edition or role claims.

## Inspect and toggle features

```sh
plane-cli workspace-feature get --api-version v2 --workspace TEAM
plane-cli project-feature get --api-version v2 --workspace TEAM --project ENG
plane-cli workspace-feature update --api-version v2 --workspace TEAM --is-wiki-enabled=false --dry-run
plane-cli workspace-feature update --api-version v2 --workspace TEAM --is-wiki-enabled=false --yes
plane-cli project-feature update --api-version v2 --workspace TEAM --project ENG --is-epic-enabled=true --yes
plane-cli workspace-feature update --api-version v1 --workspace TEAM --wiki=false --yes
```

All feature mutations require `--yes`. Updates send only supplied fields, with no follow-up GET; compare the native returned values yourself. Feature responses declare no one-time secrets or signed upload targets. A flag alone does not establish plan entitlement, and 402/409 failures are not retried or redirected.

- v1 project fields: `epics`, `modules`, `cycles`, `views`, `pages`, `intakes`, `work_item_types`.
- v1 workspace fields: `project_grouping`, `initiatives`, `teams`, `customers`, `wiki`, `pi`.
- v2 project fields: `is_automated_cycle_enabled`, `is_epic_enabled`, `is_manually_start_end_cycles_enabled`, `is_milestone_enabled`, `is_parallel_cycles_enabled`, `is_project_updates_enabled`, `is_workflow_enabled`.
- v2 workspace fields: `is_work_item_types_enabled`, `work_item_type_default_level`, `is_workitem_hierarchy_enabled`, `is_project_grouping_enabled`, `is_teams_enabled`, `is_wiki_enabled`, `is_initiative_enabled`, `is_customer_enabled`, `is_release_enabled`, `is_state_duration_enabled`, `is_pi_enabled`.

Scalar flags replace underscores with hyphens. Boolean false and integer zero are explicit values, not omissions. `work_item_type_default_level` accepts any integer; no enum/rank translation is invented. Note the exact spelling `is_workitem_hierarchy_enabled`. Unknown body keys are passed through without being translated into known toggles; upstream may ignore them. Body-file and field flags are mutually exclusive.

Changing `is_work_item_types_enabled` changes type/property write surfaces for every client. Inspect the returned mode; the CLI neither migrates callers nor retries on the other surface. This slice reuses the existing `workspace-feature get` handler rather than adding another mode-read operation.

### Bounded source discrepancy

The v2 project-feature overview advertises sparse reads, but the dedicated GET documents no `fields` parameter or requestable field list. `project-feature get` therefore exposes **no `--fields` option**. Dedicated operation contracts also take precedence over overview-wide grant shaping claims for empty DELETE responses. For exact selected-version input shapes, use `plane-cli schema <command> <action> --api-version v1|v2` offline.

Evidence: [roles](https://developers.plane.so/api-reference/v2/roles/overview), [schemes](https://developers.plane.so/api-reference/v2/permission-schemes/overview), [permissions](https://developers.plane.so/api-reference/v2/permissions/overview), [grants](https://developers.plane.so/api-reference/v2/permission-grants/overview), [pagination](https://developers.plane.so/api-reference/v2/pagination), [project GET](https://developers.plane.so/api-reference/v2/project-features/get-project-features), [workspace features](https://developers.plane.so/api-reference/v2/workspace-features/overview). Offline provenance records retain independently authored minimal contracts and captured-source hashes, not whole upstream documents.
