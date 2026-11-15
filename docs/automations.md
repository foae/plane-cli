# Automations

Automation commands use API v2 and explicit scope groups. Project scope accepts a project UUID or bare identifier such as `ENG`; workspace scope has no project path. No name lookup, feature change, workspace-mode change, graph repair or association emulation runs implicitly.

```sh
plane-cli automation project list --api-version v2 --workspace my-team --project ENG
plane-cli automation workspace get --api-version v2 --workspace my-team --id "$AUTOMATION_ID"
plane-cli automation project node list --api-version v2 --workspace my-team --project ENG --automation "$AUTOMATION_ID"
plane-cli automation workspace activity list --api-version v2 --workspace my-team --automation "$AUTOMATION_ID" --count=false
```

## Commands and inputs

Both `automation project` and `automation workspace` provide:

| Resource | Actions | Parent / target |
| --- | --- | --- |
| Automation | `list`, `get`, `create`, `update`, `delete`, `set-status` | `--id` for a target |
| `node` | `list`, `get`, `create`, `update`, `delete`, `regenerate-webhook-secret` | `--automation`, then `--id` for a target |
| `edge` | `list`, `get`, `create`, `update`, `delete` | `--automation`, then `--id` for a target |
| `activity` | `list`, `get` | `--automation`, then `--id` for a target |

Every mutation requires `--yes`, including create, update, status and secret rotation. `--dry-run` validates locally and reports confirmation requirements without credentials or requests. All commands support JSON output; there is no automation table projection. Use `schema automation project node create --api-version v2` (or the corresponding full command path) for the offline input schema.

Use `--body-file PATH` or `--body-file -` for a native JSON object. Scalar body fields also have kebab-case flags; do not combine body-field flags with a body file. Arrays such as `project_ids` use a body file. Explicit false, zero, empty strings and empty arrays remain distinct from omission.

- Automation create requires `name` (maximum 255 characters) and `scope` (`work-item`). Create/update accept `description` and `project_ids`, an array of UUID strings. Update also accepts `name` and `scope`.
- Node create requires `handler_name` (maximum 100 characters), `name` (maximum 255) and `node_type` (`trigger`, `action` or `condition`). Create/update accept string `config` and boolean `is_enabled`; update accepts the same node fields optionally. No handler catalog or handler-specific config schema is invented.
- Edge create requires UUID `source_node_id` and `target_node_id`; create/update accept integer `execution_order`. Update can replace either endpoint. Graph-validity rules are enforced by Plane, not guessed locally.
- `set-status` requires boolean `is_enabled`; it returns HTTP 204 with `data:null`. It is an explicit action, not an automatic second phase of create/update.

```sh
plane-cli automation workspace create --api-version v2 --workspace my-team \
  --name 'Triage new bugs' --scope work-item --yes
plane-cli automation project set-status --api-version v2 --workspace my-team \
  --project ENG --id "$AUTOMATION_ID" --is-enabled=false --yes
```

A newly created automation is draft and disabled. Its bot/version/run metadata remains native response data. Run IDs and scheduling timestamps are not polling handles: these captured operations contain no run submission, accepted-job contract or get-status endpoint.

## Filters and pagination

List filters are resource-specific:

- Automations: `--is-enabled`, `--is-global`, `--name`, `--scope`, `--search`, `--status`.
- Nodes: `--handler-name`, `--is-enabled`, `--name`, `--node-type`, `--search`.
- Edges: `--source-node-id`, `--target-node-id`.
- Activities: `--created-at--gt`, `--field`, `--verb`. The double hyphen preserves the native `created_at__gt` operator; this is not a PQL expression.

All lists support `--count`, `--order-by`, `--per-page` (1–200), `--offset` (0–10000), `--fields` and explicit `--all` / `--max-pages`. One page preserves the complete native envelope. `--all` returns native pages without dropping metadata or inferring completion from page length. Cursor continuation follows the [shared v2 pagination contract](https://developers.plane.so/api-reference/v2/pagination): `--paginate cursor` and `--cursor TOKEN`. The operation pages advertise cursor mode but omit the continuation parameter/example; this additional contract comes from the shared guide, not an invented automation endpoint.

Sparse `--fields` is exposed only where each operation documents it, including CRUD writes/deletes. `set-status` and rotation have no documented shaping parameter. Invalid field names remain server errors.

## One-time node webhook secrets

`node regenerate-webhook-secret` returns its declared `secret` in JSON by default. Treat stdout as sensitive. The safer opt-in delivers the secret-bearing JSON to an owner-only file and redacts the stdout copy:

```sh
plane-cli automation workspace node regenerate-webhook-secret --api-version v2 \
  --workspace my-team --automation "$AUTOMATION_ID" --id "$NODE_ID" \
  --secret-output-file ./node-secret.json --yes
```

The shared delivery helper preflights protected storage before mutation and uses atomic no-clobber installation. Replacing an existing file requires `--overwrite --yes`. Delivery failure after remote success returns exit 6 with `remote_changed:true` and `secret_delivered:false` where no delivery succeeded. A file already delivered remains delivered even if later stdout metadata fails. The CLI never retries rotation, falls back to stdout, or claims the old secret can be recovered from a read. Explicitly inspect the node and arrange another authorized rotation when needed. Old-secret invalidation, replay/idempotency and secret lifetime are not specified by these captures.

## Source conflicts and limitations

The reviewed implementation follows explicit request-field declarations over contradictory examples; native responses, including nulls, are preserved except ordinary `secret` and `secret_plaintext` keys, which the shared delivery policy redacts recursively. Only explicit rotation intentionally delivers `/secret`:

- `project_ids` is a flat array of UUID strings in the field definitions and JavaScript examples, but cURL/Python examples contain nested arrays: [project create](https://developers.plane.so/api-reference/v2/project-automations/create-project-automation), [project update](https://developers.plane.so/api-reference/v2/project-automations/update-project-automation), [workspace create](https://developers.plane.so/api-reference/v2/workspace-automations/create-workspace-automation), [workspace update](https://developers.plane.so/api-reference/v2/workspace-automations/update-workspace-automation). Nested arrays are rejected locally.
- The [project overview](https://developers.plane.so/api-reference/v2/project-automations/overview) says Plane ignores request `project_ids`; the project create/update field prose says it replaces associated projects. The documented write field is exposed unchanged, with no promise that the project endpoint honors it and no emulated association writes. Workspace association behavior is separate; its overview says out-of-workspace IDs are dropped.
- Node `config` is declared a string but examples use null. Inputs follow the string declaration, not an inferred nullable/object config contract: [project create](https://developers.plane.so/api-reference/v2/project-automations/create-automation-node), [project update](https://developers.plane.so/api-reference/v2/project-automations/update-automation-node), [workspace create](https://developers.plane.so/api-reference/v2/workspace-automations/create-automation-node), [workspace update](https://developers.plane.so/api-reference/v2/workspace-automations/update-automation-node).
- Node create requires an enumerated `node_type`, yet response examples contain null. That does not broaden accepted input. Create automation examples also show run counts/timestamps despite draft status; these are preserved, not interpreted as completed execution.
- PATCH prose refers generically to nullable fields without naming any. Optional does not establish nullable. Deletion cascade, reversibility, enabled-graph edit rules and specific graph conflict rules are not documented; every mutation is conservatively confirmed.

Permission/entitlement/conflict errors retain the established shared HTTP status and exit classification. Arbitrary upstream error detail and native codes are intentionally not exposed by the current shared error envelope. A 404 is not reclassified as local version unavailability. No error triggers a retry, feature write or rollback; a lost mutation response remains outcome-unknown.

The 40 captured automation sources cover 38 operations plus two overviews, not all P09 supplemental v1 families. They document no automation variable/history API beyond activity reads, no AI agent/run/activity submission contract, no integration credential/connection/import-job contract, and no workflow approval/state-transfer contract. Those supplemental gaps require their separate evidence review; this automation implementation does not silently exclude or claim to complete them.
