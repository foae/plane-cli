# Workflows

Workflow commands use API v2. Select the scope explicitly; the CLI never changes workspace governance to make a request work.

```sh
plane-cli workflow project list --api-version v2 --workspace my-team --project ENG
plane-cli workflow workspace get --api-version v2 --workspace my-team --id "$WORKFLOW"
plane-cli workflow workspace get-usage --api-version v2 --workspace my-team --id "$WORKFLOW"
```

Project selectors accept a UUID or bare identifier. Nested resources use `--workflow`; hooks also require `--transition`. An individual resource uses `--id`. For project chain states this is the workflow-state entry id; for workspace chain states this is the catalog state id, not the chain entry id.

## Commands and safety

Under `workflow project`:

- `list`, `get`, `create`, `update`, `delete`, `preview-fallback`.
- `state list|get|create|update|delete`.
- `transition list|get|create|update|delete`.
- `activity list|get`.
- `hook list|get|create|update|delete|list-executions|regenerate-secret`.

Under `workflow workspace`:

- `list`, `get`, `create`, `update`, `delete`, `get-usage`.
- `state list|get|add|update|remove|mark-default`.
- `transition list|get|create|update|delete`.
- `activity list|get`.
- `hook list|get|create|update|delete|list-executions|regenerate-secret`.

Every actual mutation requires `--yes`, including creates, updates, mark-default and secret rotation. Use `--dry-run` to inspect the method, URL and confirmation requirement without a request. JSON bodies use `--body-file path.json` or `--body-file -` for stdin. Scope-specific body schemas are available offline through the command's schema/help.

`workflow project preview-fallback` is explicitly a read-only POST and does not require `--yes`. Its body can select `type_id`, `new_type_id` or `workflow_id` according to the documented preview contract. A result with `total: 0` is not validation of the supplied workflow: an unknown or empty target can also produce zero. No mutation or mode switch follows a preview automatically.

## Editing graphs

Read the current workflow before editing. Catalog workflow detail includes `states`, transitions, enabled hooks and `referenced_resources`. List responses carry counts rather than that full graph.

Catalog `update` without `states` edits metadata only. With `states`, it destructively replaces the entire chain: omitted states leave, sequence and creation defaults are rewritten, and `default_state_id` is required. Omitted `allow_issue_creation` becomes `true`, including on retained states. `is_active` is ignored in this form. Rename separately: duplicate names return `409 conflict` for metadata updates but documented `500 server_error` for chain replacement. Replacement leaves transitions referring to dropped states in place; remove those transitions deliberately before replacing the chain.

Catalog `state remove` differs: it refuses the last state, requires a remaining default when removing the default, checks stranded work items, then checks transition dependencies. The catalog state itself is not deleted. Supply the documented optional DELETE body with `new_default_state_id` and `state_mapping` as needed. Neither command guesses relocation targets or automatically retries a conflict.

**A successful chain replacement or state removal response acknowledges the mutation only. Background relocation completion is unverified.** Above 500 work items per project and work item type, moves can continue in the background and immediate reads can still show old states. The CLI preserves the documented HTTP `200`/`204` and native response; the standard envelope's `meta.status: completed` describes the HTTP request, not background relocation. These sources provide no relocation job id or status endpoint. Do not automatically repeat the mutation to wait for relocation.

Project state `type` accepts `transition` or `approval`. Catalog state update deliberately has a different contract: upstream saves other changed type strings, deletes outgoing transitions and approvers, and applies only `type` when it changes. Send `allow_issue_creation` separately. The CLI does not normalize these contracts into a shared enum or simulate graph changes.

Transition creation refuses duplicate source/target pairs. Retargeting an existing catalog transition does not perform that uniqueness check, so duplicate pairs can remain. Source state cannot be moved by catalog transition update; delete and create deliberately instead. Approval settings and nullable transition/rejection targets are passed through without inferred graph validation.

## Permissions, governance and conflicts

Project routes do not redirect to catalog routes. Catalog resources use governed workspaces; ungoverned access can return `409 states_managed_at_project`. Permission checks precede that governance check, and item-changing catalog routes look up the item before either, so `403` and `404` must not be rewritten as governance conflicts. Plan checks and migration write pauses also remain upstream.

Conflict and permission responses preserve the established safe HTTP status and CLI error classification; upstream detail text and API-specific codes are not exposed. Failed or interrupted mutations are not automatically retried, rolled back or declared completed; mutation-outcome uncertainty remains visible.

## Hooks and one-time signing secrets

Create bodies explicitly choose immutable `handler_name` (`send_webhook` or `run_script`) and `phase` (`pre` or `post`). A failed pre-hook blocks the move; post-hook failure does not undo it. Project scripts can be project-scoped or workspace-wide; catalog scripts must not be tied to a project. The CLI does not substitute scripts or relax server permissions.

Webhook creation returns the one-time signing secret at `config.secret_plaintext`; bodyless `regenerate-secret` returns the replacement at `secret`. Rotation invalidates the old secret immediately. Ordinary hook reads omit the signing secret; nested catalog workflow detail can contain a masked `config.secret`, which is a distinct documented read shape, not a retrievable plaintext secret.

These operations use the shared intentional-secret JSON delivery contract. Opt into owner-only file delivery with `--secret-output-file path.json`; replacing an existing destination additionally requires `--overwrite --yes`. Without a file, the intentional secret-bearing result is emitted as JSON. Store it securely. The CLI never silently rotates after delivery failure. Explicitly run the corresponding scoped `hook regenerate-secret --yes` with the affected ids if recovery requires a new signing secret. `run_script` creation uses ordinary output and does not expect a signing secret.

`--fields` is rejected locally for `send_webhook` creation in both scopes: a sparse response could discard the one-time signing secret after the hook is created. Ordinary output and protected-file receipts also redact header values marked `is_secret` or named `Authorization`, `Proxy-Authorization`, `X-API-Key`, `Cookie`, `Set-Cookie` or `X-Plane-Signature`; nonsecret headers remain visible.

Supplying either secret-delivery flag for `run_script` creation is a local usage error, not silently ignored. Ordinary workflow output recursively redacts signing-secret fields, including masked `config.secret` values in nested detail; intentional creation/rotation delivery is the explicit exception.

On webhook update, omitted URL/headers retain their stored values, but a supplied header array replaces the stored list. A secret header can retain its encrypted value by matching an existing key case-insensitively and omitting `value`; other headers need a value. Script config merges supplied keys. Updates never rotate signing secrets.

## Lists and activity

List filters and sparse fields are native query parameters. Use `--all` and `--max-pages` for bounded aggregation. To use cursor pagination on a chain-state or hook list whose default ordering is ineligible, specify `--paginate cursor --order-by created_at`; the CLI does not silently change ordering. Activity supports `--created-at--gt` for incremental reads; records can appear after the mutation because activity writes occur in the background.

Exact source contracts and provenance are retained in `api/sources/p09-workflows-*.json` and the catalog. Examples and tests use synthetic documentation fixtures only.
