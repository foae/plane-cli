# Customers

Customers are workspace CRM records, not intake work items. Select the API generation explicitly when it differs from the configured connection. Every command takes `--workspace`; nested records additionally take `--customer`.

## Commands

| Command | Actions | Versions |
| --- | --- | --- |
| `customer` | `list`, `get`, `create`, `update`, `delete` | v1, v2 |
| `customer` | `upsert` | v2 |
| `customer property` | `list`, `get`, `create`, `update`, `delete` | v1, v2 |
| `customer property-value` | `list` | v1, v2 |
| `customer property-value` | `get`, `update` | v1 |
| `customer property-value` | `set` | v2 |
| `customer request` | `list`, `get`, `create`, `update`, `delete` | v1, v2 |
| `customer work-item` | `list` | v1, v2 |
| `customer work-item` | `add`, `remove` | v1 |
| `customer work-item` | `manage` | v2 |

Customer detail actions identify their record with `--customer`; property definitions use `--property`; request detail uses `--request`; v1 work-item removal uses `--work-item`. A property definition belongs to the workspace, while property values belong to a customer. There is no intake conversion or undocumented customer membership endpoint.

Use `--body-file FILE` (or `--body-file -` for stdin) for the exact JSON body. Do not combine a body file with body-field flags. Omitted keys stay omitted, explicit nullable values stay `null`, and empty strings, objects, arrays, `false` and zero are not discarded. Structured fields are supplied through the body file. The selected generation's schema rejects cross-generation payloads rather than converting them.

Common customer list/get supports `--output table`, projecting `id`, `name`, `stage` and `contract_status` without relation fetches. JSON remains the default and preserves native fields. Lists expose `--all` only where the reviewed endpoint has a paginator; otherwise the native single response is returned. v2 customer work-item listing is offset-only with fixed newest-created ordering: it accepts `--customer-request-id`, `--per-page`, `--offset`, `--count`, `--fields` and `--expand`, but not search, custom ordering or cursor mode.

## Recipes

Create a customer using a v2 JSON document:

```sh
printf '%s\n' '{"name":"Acme","email":"hello@example.com","description":null,"employees":0}' |
  plane-cli customer create --api-version v2 --workspace example --body-file -
plane-cli customer list --api-version v2 --workspace example --search Acme --output table
```

Create a workspace property definition, then set a customer's answers separately:

```sh
printf '%s\n' '{"display_name":"Tier","property_type":"OPTION","options":[{"name":"Enterprise","color":"#123456"}],"default_value":[]}' |
  plane-cli customer property create --api-version v2 --workspace example --body-file -
printf '%s\n' '{"values":{"PROPERTY_ID":["OPTION_ID"]}}' |
  plane-cli customer property-value set --api-version v2 --workspace example --customer CUSTOMER_ID --body-file -
plane-cli customer property-value list --api-version v2 --workspace example --customer CUSTOMER_ID
```

The v2 value setter sends a `values` object in a POST and has a documented empty `201` response. Its dynamic values are preserved without inventing property-type coercion or a universal null-clearing rule. The v1 value updater instead targets one `--property`, PATCHes `{"values":[]}`, and returns `204`.

Link work items directly, or track a request and its own associations:

```sh
printf '%s\n' '{"add":["WORK_ITEM_ID"]}' |
  plane-cli customer work-item manage --api-version v2 --workspace example --customer CUSTOMER_ID --body-file -
printf '%s\n' '{"name":"Export report","description":null,"work_item_ids":["WORK_ITEM_ID"]}' |
  plane-cli customer request create --api-version v2 --workspace example --customer CUSTOMER_ID --body-file -
plane-cli customer work-item list --api-version v2 --workspace example --customer CUSTOMER_ID --customer-request-id REQUEST_ID
printf '%s\n' '{"work_item_ids":[]}' |
  plane-cli customer request update --api-version v2 --workspace example --customer CUSTOMER_ID --request REQUEST_ID --body-file - --yes
```

A request update's `work_item_ids` replaces visible active associations; omitting it preserves them and `[]` clears them. Replacing that association list requires `--yes`. v2 may skip inaccessible, archived or outside-workspace IDs rather than rejecting the request; inspect the returned associations. Customer work-item reads include direct and request-derived links, subject to upstream visibility, without fetching each work item separately. v1 direct linking sends `issue_ids` to `/issues/`; v2 sends `add`/`remove` to `/work-items/`. Nonempty `remove`, v1 `remove`, and all deletes require `--yes`. Removing a link is not deleting the linked work item.

v2 upsert requires `name`, `external_source` and `external_id`. The native `X-Plane-Upsert` outcome is exposed in JSON metadata; serialize writes per external key because concurrent upserts can create duplicates. v1 descriptions and logo properties are objects, while v2 descriptions and editor-managed logo descriptors are strings. Property write `options` are object arrays in the exact v2 operation contract, despite the overview's string-array description.

HTTP `402` remains an upstream failure, not a successful empty result or proof of a particular edition. No capability preflight, automatic version fallback or retry is performed. Some v1 pages mention optional search/request filters but supply no selector names; the CLI does not invent those undocumented parameters.

See [API coverage](design/api.md) and [offline evidence](../api/README.md) for provenance and the distinction between implemented and verified operations.
