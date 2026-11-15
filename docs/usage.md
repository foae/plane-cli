# CLI usage

P00-P02 implement offline discovery, named connections, persistent credentials, user/bot OAuth and the safe API transport foundation. P03 exposes identity, projects, work items and state/label definitions. P04 adds scheduling, estimates, intake, recurrence and work-item collaboration for their documented API generations. Supplemental v1 evidence gaps remain listed in [API contract gaps](reference/api-contract-gaps.md); this is not full public API coverage.

Build with Go 1.27.4 and `make build`, or install from this source checkout with `go install ./cmd/plane-cli`. Generated metadata is committed and embedded: the installed binary does not need repository files. Contributor setup and checks are in [development](development.md).

```sh
plane-cli --help
plane-cli version
plane-cli --version
plane-cli commands --api-version v2
plane-cli commands schema --api-version v1
plane-cli schema commands --api-version v2
plane-cli schema schema --api-version v1
```

No arguments, `--help` and `help [command-path...]` print human-readable help. Other successful commands emit one JSON envelope with `schema_version`, `data`, and local `meta.command`/`meta.status`. Local metadata never invents HTTP status or API response information. `version` and `--version` produce the same build metadata; source builds report `dev`/`unknown` until release metadata is supplied by the P10 packaging workflow.

`commands [GROUP]` lists only implemented command summaries and explicit version availability. `schema <command-path...>` resolves the entire canonical path; its `.data.descriptor.schema` is the same JSON Schema used to validate the command's inputs. The input object has `arguments` (positional tokens) and `flags` (explicit values, plus the effective API version for resource commands). Discovery requires explicit `--api-version v1|v2`.

Unknown command paths (including help topics), invalid UTF-8 arguments or unknown flags return exit 2 and one safe JSON error on stderr, with empty stdout. Missing required flags are named without their values. A known descriptor unavailable in the selected version returns exit 7. Output failure or invalid embedded schemas return exit 1; interrupt cancellation returns 130 unless the command has already completed successfully. Known remote mutation followed by delivery failure returns exit 6.

Schemas resolve only embedded resources. JSON parsing rejects duplicate keys and trailing values while preserving exact numbers and the distinction between omitted properties, null and explicit zero values. No schema is fetched at runtime.

## Connections and environment credentials

```sh
plane-cli connection add local --base-url http://plane.example.net/plane --api-version v2 --auth-mode external
plane-cli connection use local
plane-cli connection show
plane-cli connection list
plane-cli auth status
plane-cli connection delete local --yes
```

Configuration lives in the OS user config directory under `foae-plane-cli`, or `PLANE_CLI_CONFIG_DIR`. Directories must be owner-only (0700); files and stable locks are 0600. Symlink/unsafe parents are refused; use a canonical path when a system directory is an alias (such as `/private/tmp` on macOS). Concurrent updates preserve unrelated connections. Stable lock files remain after deletion so existing waiters and new callers cannot acquire different lock inodes. Profile names are opaque map keys, not filesystem paths. Unknown configuration fields are rejected rather than silently erased; records larger than 8 MiB are refused before replacement.

Settings resolve explicit flag, nonempty `PLANE_CLI_*` environment, selected connection, then safe default. API generation and destination have no implicit default. `connection show` reports nonsecret values and their sources without reading tokens. List/use/delete ignore unrelated malformed environment overrides. HTTP works without opt-in; only adding an HTTP connection or changing HTTPS to HTTP emits a structured informational warning.

Resource dispatch resolves the API version from flags, environment or the selected connection before choosing its schema. A fully specified invocation without an explicit connection can run when local configuration is unavailable. A selected named connection never silently falls back. Dry-run validates route segments and reports the target URL and whether a body is present, without reading credentials.

`api-key` uses `PLANE_CLI_API_KEY`; `bearer`, `oauth-user` and `oauth-bot` accept `PLANE_CLI_ACCESS_TOKEN`. Both overrides together are rejected. An access-token override is externally managed: it never refreshes or writes stored credentials. `external` ignores both and sends neither header, allowing an operator proxy to inject authentication; status says `externally_managed`, not authenticated. Backends never silently fall back.

Transport honors standard proxy variables, verifies HTTPS certificates and appends `--ca-file` roots. API redirects are refused. Total/request defaults are 5m/30s; both require positive durations. There are no automatic retries: transport uses HTTP/1 only and disables keep-alive pooling to prevent Go's HTTP/2 stream retries and stale pooled-connection replays. JSON responses retain native fields and exact numbers, are limited to 16 MiB, and are staged before stdout. Server-error responses to mutations report an unknown outcome. Arbitrary upstream error text is suppressed rather than exposing reflected secrets.

## Current identity

`user me` calls the selected generation's `GET /api/v1/users/me/` or `GET /api/v2/users/me/` without a workspace or project. It preserves the native response, including v2 `principal_kind` and `scopes` when returned. V1 documents `profile:read`; v2 declares no fine-grained scope but requires OAuth tokens to carry a read scope.

```sh
plane-cli schema user me --api-version v2
```

```sh manual
plane-cli user me --base-url https://plane.example.net --api-version v2 --dry-run
plane-cli user me --connection personal
```

Contracts: [v1 current user](https://developers.plane.so/api-reference/user/get-current-user.md), [v2 current user](https://developers.plane.so/api-reference/v2/users/get-current-user.md). Deterministic request fixtures cover both generations; this is not completed P03 coverage or planner approval.

## Project and work-item lifecycle

Discover the selected version before sending requests; schemas expose native writable fields and scope requirements:

```sh
plane-cli schema project create --api-version v2
plane-cli schema work-item create --api-version v2
plane-cli schema state list --api-version v2
```

The following examples use placeholders. Configure the connection and protected credentials as above, then replace workspace/project/state/label IDs with values returned by your server:

```sh manual
plane-cli project list --connection personal --workspace example
plane-cli state list --connection personal --workspace example --scope project --project PROJECT_ID
plane-cli label list --connection personal --workspace example --project PROJECT_ID
plane-cli work-item create --connection personal --workspace example --project PROJECT_ID --body-file item.json
plane-cli work-item get --connection personal --workspace example --project PROJECT_ID --id ITEM_ID
plane-cli work-item update --connection personal --workspace example --project PROJECT_ID --id ITEM_ID --body-file update.json
plane-cli work-item list --connection personal --workspace example --scope project --project PROJECT_ID --all
plane-cli work-item delete --connection personal --workspace example --project PROJECT_ID --id ITEM_ID --yes
```

For v2, `item.json` can contain `{"name":"Example task","state_id":"STATE_ID","label_ids":["LABEL_ID"]}`; `update.json` can contain `{"name":"Updated task"}`. Use the version's schema rather than renaming v1 fields by analogy. Body files contain the native body, not a CLI envelope; `--body-file -` reads stdin. A body document and convenience body-field flags are mutually exclusive. Omitted values remain absent; explicit null, false, zero, empty strings and arrays are preserved only where the operation's schema permits them.

For resource commands, `--timeout` bounds body input, authentication and API traversal together; waiting for stdin does not start a separate allowance. Read-only advanced search remains a read even though its HTTP method is POST: failures do not imply a remote mutation.

Ordinary JSON output wraps the untouched native response under `data`. One-page lists therefore keep native `results` (v1) or `data` (v2) nesting. `--all` instead places an array of complete native page responses under the outer `data`; it never flattens away pagination metadata or unknown fields. It stages every page before stdout, with a 64 MiB aggregate bound, 1,000-page default, explicit `--max-pages`, and rejection of repeated/nonprogressing continuations. A failed later page leaves stdout empty. V2 offset traversal stops at the documented offset ceiling rather than silently changing pagination mode.

Aggregation failures include `error.progress` with `mode: "all"`, the number of fully staged `pages`, and `complete: false`. This is progress, not delivered data; continuation tokens are never included. Null or missing continuation flags are protocol errors, not evidence that traversal finished.

Declared list/get commands accept `--output table` for a fixed native-field projection without relation fetches. JSON remains the default. Native bulk operations preserve indexed results; any failed row yields `meta.status: "partial"` and exit 6. Upsert metadata exposes the documented `X-Plane-Upsert` outcome when returned. Deletes and destructive archive operations require `--yes`; `--dry-run` validates locally without credentials or HTTP.

V1 `epic` commands expose the separately documented epic API: `create`, `list`, `get`, `update`, `delete`, `add-work-items`, and `list-work-items`. They require workspace/project scope; item operations also require `--id`. Membership writes use a body with `work_item_ids`. V2 has no separate documented epic route in this packet; use its work-item type fields rather than assuming the v1 route exists.


## Persistent login and OAuth

The following manual examples require operator-supplied protected files and app registration; the offline documentation checker does not execute `sh manual` blocks.

```sh manual
plane-cli connection add personal --base-url https://plane.example.net --api-version v2
plane-cli auth login --connection personal --api-key-file /protected/token
plane-cli auth status --connection personal
plane-cli auth migrate --connection personal --backend keyring --yes
plane-cli auth logout --connection personal --yes
```

Token/client-secret/callback input files must be regular, owner-only files in an owner-only directory with safe ancestors, without symlinks; `-` reads stdin and honors command cancellation. Secret values and callback URLs are not accepted on argv. File storage is owner-only plaintext at rest; keyring storage is opt-in (macOS Keychain or Linux Secret Service), with no fallback when unavailable. Explicit `env` backend login fails before any exchange. Logout removes local credentials, not remote grants.

Native keyring availability and unlock policy depend on the operating system. Linux cancellation terminates and reaps the local helper, but cannot roll back a write already accepted by Secret Service. macOS verifies writes by reading them back; bundles exceeding the `security` interactive-command limit are rejected rather than truncated. Choose file storage explicitly if that limit prevents login.

If login reports that new credentials were saved but cleanup failed, the committed credential remains usable; follow the reported `auth logout --yes` repair guidance rather than repeating the token exchange. A failed staged write preserves the previous credential, and a subsequent login/import/migration retries only staged cleanup first. User refresh exchanges are never replayed after an uncertain outcome; transient bot-grant failures can be retried by a later invocation.

OAuth requires a registered confidential app and its exact registered callback URI. Configure a connection with `--auth-mode oauth-user` or `--auth-mode oauth-bot`. User login uses `--flow user`; bot login uses `--flow bot`. Supply `--client-id`, `--client-secret-file` and `--redirect-uri`; bot login may instead use an existing `--installation-id`. Optional `--scopes` is passed to the provider. Nonempty `PLANE_CLI_OAUTH_CLIENT_ID`, `PLANE_CLI_OAUTH_CLIENT_SECRET`, `PLANE_CLI_REDIRECT_URI`, `PLANE_CLI_SCOPES` and `PLANE_CLI_INSTALLATION_ID` supply defaults.

Changing the OAuth client, redirect, scopes or installation during explicit login rebinds the connection and removes its previous credentials before consent. Check environment defaults before doing so; cancelling consent does not restore the old session.

Local consent requires an HTTP loopback URI with an explicit nonzero port. The listener binds before consent instructions appear on stderr. Login, completion and status honor the effective command timeout; OAuth requests honor effective request-timeout and CA overrides. The command timeout also bounds local consent waiting. For a different registered redirect, use headless completion:

```sh manual
plane-cli auth login --connection personal --flow user --headless \
  --client-id registered-app --client-secret-file /protected/app-secret \
  --redirect-uri https://callback.example/callback
plane-cli auth complete --connection personal --login-id LOGIN_ID --callback-file -
```

Headless login returns a pending login ID and consent URL, not authenticated success. Consent URLs intentionally contain OAuth state; treat them as sensitive. Pending attempts expire after ten minutes and are single-use, bound to connection identity and generation. Expired or consumed attempts require a new login. No device-grant or public-client PKCE support is assumed.

Stored user sessions refresh automatically with their persisted app credentials; bot sessions renew using client credentials and installation ID, not refresh tokens. Connection-scoped locks serialize renewal, logout and rebinding across processes. Failed persistence after exchange returns exit 6 and fresh-login guidance rather than replaying a consumed refresh token. Failed cleanup retains references for `auth logout --yes` repair. Changing the destination, auth mode or backend removes old bound credentials; use `auth migrate` to preserve credentials when changing backends.

Fake providers and controlled credential backends verify these protocols. Native keyring availability and deployment-specific app registration remain unverified.


## Scheduling and collaboration

`cycle`, `module`, `milestone`, `estimate`, `estimate point`, `intake`, `recurring-work-item`, `work-item-relation`, `work-item-comment`, `work-item-link`, `work-item-activity`, `work-item-worklog`, `work-item-subscriber` and `work-item-vote` expose only their documented API generations. Use each leaf's schema for the exact input fields and parent scope:

```sh
plane-cli schema intake create --api-version v1
plane-cli schema intake create --api-version v2
```

```sh manual
plane-cli recurring-work-item --help
plane-cli intake list --api-version v2 --base-url http://127.0.0.1:1 --workspace example --project p --status-in=-2,0 --dry-run
```

V1 intake creation nests work-item fields under `issue`; v2 uses flat fields. Deprecated inbox documentation is tracked as an alias, not a second command surface. Recurring schedules preserve upstream date strings and recurrence fields; `enabled` is not a writable field. V1 relation lists return a relation-name map rather than an aggregate page, so `--all` is unavailable.

Comments and links preserve HTML/text without rendering it. Worklog updates distinguish zero duration from omission. Subscriber updates require disjoint `add`/`remove` sets; removals require `--yes`. Subscriber by-identifier commands use a workspace and work-item identifier without project scope. Bulk comment failures return the native result and partial-success exit 6; specify `all_or_none` inside the JSON body, alongside `items` or `ids`.

Scheduling commands retain version-specific membership, archive and transfer routes. Transfers require `--yes` and the documented destination in the request body. Estimate points are a subresource of `estimate`, not estimate-system writes. V1 estimate updates/deletes target the project's active system without `--id`. V2 cycle create/update dates use ISO8601 timestamps; upsert has conflicting upstream date prose, so it preserves the supplied string without imposing either interpretation. Module/milestone date fields retain their documented date-only format.

Cycle timezone validation embeds IANA timezone data and does not require host zoneinfo. V2 collaboration lists accept opaque `--cursor` values for manual continuation. Automatic traversal is bounded with `--max-pages`, not an item-count limit.

The [packet router](implementation/README.md) identifies remaining work. Deterministic fake-server acceptance does not establish live Plane edition compatibility.

## Types and custom properties

Project commands use `work-item-type`, `work-item-property`, `work-item-property-option` and `work-item-type-property`. Their workspace counterparts start with `workspace-`; `work-item-property-context` is workspace-scoped. V1 property values use `work-item-property-value`; v2 values belong in a work item's `custom_fields` body.

Read `workspace-feature get` and `work-item-type-governance get --type TYPE_ID` explicitly when needed. Mutations do not discover modes, fetch schemas, change governance, or retry against another scope. A 409 is returned through the existing sanitized API-error envelope with `http_status: 409`.

This v2 recipe needs a configured connection, `jq`, project-managed types, and an existing work item of the selected type. Set `WORKSPACE`, `PROJECT_ID`, `TYPE_ID` and `WORK_ITEM_ID` to your own values. It creates and attaches an optional text property, reads the resulting schema, then updates that property on the work item:

```sh manual
set -eu
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
printf '%s\n' '{"display_name":"Runbook note","property_type":"TEXT","is_required":false,"is_multi":false}' > "$tmp/property.json"
plane-cli work-item-property create --api-version v2 --workspace "$WORKSPACE" --project "$PROJECT_ID" --body-file "$tmp/property.json" > "$tmp/created.json"
property_id=$(jq -er '.data.id' "$tmp/created.json")
property_name=$(jq -er '.data.name' "$tmp/created.json")
jq -n --arg id "$property_id" '{properties:[$id]}' > "$tmp/attach.json"
plane-cli work-item-type-property attach --api-version v2 --workspace "$WORKSPACE" --project "$PROJECT_ID" --type "$TYPE_ID" --body-file "$tmp/attach.json"
plane-cli work-item-type get-schema --api-version v2 --workspace "$WORKSPACE" --project "$PROJECT_ID" --id "$TYPE_ID" > "$tmp/schema.json"
jq -e --arg name "$property_name" '.data.custom_fields[$name]' "$tmp/schema.json"
jq -n --arg name "$property_name" '{custom_fields:{($name):"See the service runbook"}}' > "$tmp/value.json"
plane-cli work-item update --api-version v2 --workspace "$WORKSPACE" --project "$PROJECT_ID" --id "$WORK_ITEM_ID" --body-file "$tmp/value.json"
```

Use the returned property name, not its display name, as the `custom_fields` key. The server's type schema supplies required/cardinality and option constraints; dynamic keys and native JSON values pass through unchanged. Omitted PATCH properties keep their values. Property definitions, type attachments, context overrides and recorded values are separate resources.

Context updates require `--yes`: replacing options may delete recorded values. The upstream property overview and context overview disagree about whether an all-project/all-type default context is seeded; the CLI does not infer or create one. Workspace `is_epic` is read-only: creating a type named “Epic” does not make it a system epic.

## Pages, collections and files

`work-item-page`, `project-page`, `workspace-page`, `published-page`, `sticky` and `collection` expose source-backed operations by API generation. Use leaf help and `schema` for availability and exact fields. Page IDs and work-item/page link IDs are distinct. HTML and opaque editor metadata pass through unchanged; archive, restore and delete are explicit commands requiring `--yes`, never automatic prerequisites or retries.

Collection membership additions and access updates require `--yes` because they change permissions. Dry-run reports confirmation requirements without requiring approval or credentials.

```sh
plane-cli schema work-item-page create --api-version v2
plane-cli schema workspace-page-attachment download --api-version v2
```

`attachment`, `workspace-page-attachment`, `workspace-asset` and `user-asset` expose documented native asset operations. `upload --file PATH` performs metadata creation, credential-free signed multipart storage upload, then confirmation where that workflow is documented. Local file checks precede metadata creation. A failed later phase returns exit 6 with the asset ID and phase: inspect the asset before explicitly confirming or deleting it. There is no automatic rollback or replay.

Upload metadata preserves native JSON numbers. Filename and size come from the regular local file, not `--body-file`; v1 wiki attachment entity fields are derived from `--page` and cannot be overridden. Storage streaming uses the remaining invocation `--timeout`, not the ordinary API `--request-timeout`; raise `--timeout` explicitly for large files. A result-delivery failure after confirmation does not mean the asset is incomplete: inspect it rather than repeating the mutation.

Downloads require `--output-file`; existing destinations require `--overwrite --yes`. Bytes are streamed into an owner-only sibling temporary file and installed only after declared size checks. Storage gets no Plane API credentials and follows no redirects. Configured HTTP storage is accepted. Ordinary metadata reads redact signed targets/form fields; explicit native upload-target delivery is sensitive and should use a protected output file.

Downloads preserve stored bytes, including gzip-encoded objects. No-clobber installation requires filesystem hard-link support; unsupported destinations fail safely without replacement.

P06 is a bounded implementation, not a completed packet. Conflicting v2 collection schemas, undocumented supplemental server-mediated uploads and unresolved source contracts remain recorded in the inventory; unavailable operations are not guessed or counted as delivered. Fake HTTP acceptance verifies protocol behavior, not live edition compatibility.
