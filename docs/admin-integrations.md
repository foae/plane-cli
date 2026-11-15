# Audit, notifications, webhooks and artifacts

These administration commands use the explicitly selected v2 API. No feature is enabled, role inferred, or alternate API version probed as a precondition. JSON output preserves native resource data except declared credential redaction in ordinary webhook output. Audit actors, IP addresses, notification content and delivery payloads can still contain sensitive personal or customer data.

## Read and export

```sh
plane-cli audit-log list --api-version v2 --workspace WORKSPACE --created-after 2026-01-01T00:00:00Z --created-before 2026-02-01T00:00:00Z
plane-cli notification list --api-version v2 --workspace WORKSPACE --archived=false
plane-cli webhook log list --api-version v2 --workspace WORKSPACE --webhook WEBHOOK_UUID --paginate cursor --all --max-pages 20
```

Lists return one native page by default. `--all` preserves filters and uses documented offset/cursor continuations, with a default 1000-page limit or explicit positive `--max-pages`. The aggregate is bounded to 64 MiB, staged in protected temporary storage and emitted only when complete. A page/size limit fails without partial stdout; use smaller time windows or manual pagination. Shared v2 pagination documents `--cursor` even where a leaf page omits it. Page sizes above the upstream maximum are left for upstream clamping.

Notifications are the caller's own inbox, not workspace-wide administration. `notification get --archived=true` is required to read an archived notification; otherwise it can return 404. An audit record's `--id` is its record ID, not its `event_id`. Webhook logs use both the parent `--webhook` and target `--id` for detail reads. Credential-named fields in ordinary webhook/log JSON are redacted recursively, including authorization and signing headers. Opaque string-valued request/response headers are withheld in full; arbitrary free-text payloads remain sensitive.

## One-time webhook credentials

Every webhook mutation requires `--yes`, including creation, update and regeneration. Dry-run validates the native request and reports confirmation requirements without network or credential access.

```sh
plane-cli schema webhook create --api-version v2
plane-cli webhook create --api-version v2 --workspace WORKSPACE --body-file webhook.json --dry-run
plane-cli webhook create --api-version v2 --workspace WORKSPACE --body-file webhook.json --yes --secret-output-file webhook-secret.json
plane-cli webhook regenerate-secret --api-version v2 --workspace WORKSPACE --id WEBHOOK_UUID --yes --secret-output-file rotated-secret.json
```

A creation body can be `{"url":"https://receiver.example.invalid/events","scopes":["workitem.created"]}`. Rich and PQL filters remain native bodies; the CLI does not evaluate them. Update replaces supplied scopes/filters rather than merging them. `last_used_filter` determines which filter upstream evaluates.

Create/regenerate deliberately return `secret_key` in JSON stdout by default. Treat stdout as credential material. Ordinary list/get/update cannot recover that secret. `--secret-output-file` instead delivers the complete native result in an owner-only JSON file and emits a nonsecret receipt. File preflight happens before the mutation. Existing targets require `--overwrite` with `--yes`; symlinks and unsafe targets are rejected. A failure after remote success exits 6 with `remote_changed`, `secret_delivered`, and recovery guidance, without retry or rollback. Inspect the webhook and explicitly rotate if delivery failed. Regeneration invalidates the previous signing secret for subsequent deliveries.

Missing, empty or malformed signing credentials in a successful create/regenerate response are delivery failures, not successful issuance. No protected file is installed and no success payload is emitted. The nonsecret file receipt applies the same recursive credential redaction as ordinary reads. Aggregate limits account for the serialized, redacted bytes.

## HTML artifacts

```sh
plane-cli schema artifact create --api-version v2
plane-cli artifact create --api-version v2 --workspace WORKSPACE --body-file artifact.json --dry-run
plane-cli artifact update --api-version v2 --workspace WORKSPACE --id ARTIFACT_UUID --body-file artifact-version.json --yes
plane-cli artifact publish --api-version v2 --workspace WORKSPACE --id ARTIFACT_UUID --yes
```

The official artifact endpoints are an unstable public proof of concept, require workspace admin/owner authorization and the Applets entitlement, and may change upstream. HTML is sent and returned as data, never rendered or executed by the CLI. `html` is required for create/update; optional `prompt` is stored upstream but not returned by reads. Create uses `project`, not `project_id`; upstream silently ignores a project outside the workspace, so inspect the returned project if scoping matters.

Update appends an HTML version rather than performing an ordinary metadata patch. Publish is a bodyless POST returning 201, including repeat publication. Save the returned anchor: detail reads do not expose it, and the documented surface has no unpublish/delete operation. Do not infer one. Official prose/response examples override contradictory OpenAPI 200 declarations for create/publish. Missing-HTML errors use a bare `detail` body rather than the usual v2 problem document.

All examples use placeholders and must be deliberately configured before execution. Acceptance uses deterministic local HTTP fixtures, not live invitations, webhook receivers, permission changes or edition compatibility probes.
