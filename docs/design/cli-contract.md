# CLI contract

Approved design, not an existing command reference. This contract must be settled before resource implementation.

## Grammar and discovery

`plane-cli <singular-kebab-case-resource> <verb> [flags]`.

Canonical groups include `connection`, `auth`, `project`, `work-item`, `state`, `label`, `cycle`, `module`, `milestone`, `estimate`, `intake`, `page`, `collection`, `initiative`, `release`, `customer`, `workflow`, `automation`, `member` and their explicitly named subresources. Actions include documented CRUD, archive, bulk, upsert, relationship and domain-specific verbs. Register the union of implemented, reviewed commands across v1/v2; do not remove a known command from the parser because a different version is selected. Discovery declares each version's availability; an unavailable invocation returns exit 7 after local version resolution but before secret lookup or HTTP. Never invent a verb with no documented operation in either version.

Parent scope uses `--workspace`, `--project`, `--work-item`, etc.; the target resource uses `--id`. IDs remain opaque/case-preserving. `--project` accepts a human project key only where the exact endpoint explicitly documents it. `work-item get-by-identifier --identifier ENG-12` uses the documented route, not an unbounded list/name search. No fuzzy selection, implicit aliases or name-to-ID requests. Nested project/workspace resource families use `--scope project|workspace` when both exist; scope is required for these families, never inferred from a default project. The API catalog fixes each command mapping before coding.

- No arguments: concise root help, exit 0, no HTTP/config writes/keyring access.
- `--help`: human-readable help, with leaf required/optional inputs, version availability, side-effect class, examples, JSON body shape and relevant next command.
- `commands [GROUP] --api-version v1|v2`: JSON summary catalog, not every nested schema.
- `schema <command-path...> --api-version v1|v2`: offline JSON Schema 2020-12 for CLI arguments/body plus method/path, response kind, scopes and destructive classification. The positional tokens are the exact canonical command path without `plane-cli`, including every nested group and the leaf verb; e.g. `schema work-item get`. No dotted/slash shorthand. Resolve the entire path, not just its first two tokens. Requires explicit version when not derivable from local settings, never credentials or network.
- `version` and `--version`: identical structured metadata; no update checks.
- Planned P10 `update check` explicitly queries CLI distribution metadata without Plane configuration/authentication. It never installs updates or runs implicitly; its output/error and network rules are in [releases](../releases.md#explicit-update-checking--planned-p10d).
- Shell completion is local/static; never network-discover workspace names on tab completion.

## Global settings and input

Global API flags: `--connection`, `--base-url`, `--api-version`, `--workspace`, `--project`, `--auth-mode`, `--timeout`, `--request-timeout`, `--ca-file`, `--output json|table`, `--yes`, `--dry-run`. Register applicable flags consistently; reject flags without meaning for a leaf. Connection precedence and external auth are in [authentication](authentication.md).

`--body-file PATH` or `--body-file -` provides the full JSON body. Common explicit field flags are a convenience, not a reduced API surface. Body-file and body-field flags are mutually exclusive; path/query/global flags remain compatible. Check `Flag.Changed`, not zero values. Omitted, null, false, zero, empty string and empty list remain distinct. Clear nullable/collection fields with explicit JSON. Reject invalid JSON, duplicate object keys, trailing values and wrong top-level shape before network access; preserve number precision. Validate known fields/types/requiredness against the pinned operation contract, but respect documented `additionalProperties`/custom-field extensibility rather than imposing an invented closed schema. Do not silently discard unknown fields.

Only one input consumer may claim stdin (credential, OAuth callback, JSON or file bytes). Conflict is a usage error before any request. Never accept secrets as flag values; use secret env vars or protected files/stdin. Input limit: 8 MiB for ordinary JSON; uploads are streamed and separately bounded by their documented operation constraints. URL path segments are escaped individually, queries use `url.Values`; reject slash/dot-segment ambiguity rather than turning IDs into paths.

Version-specific query flags map to exact documented parameters; do not invent universal filters. `--fields`/`--expand` are server-side shaping only where supported. REST filters are not PQL. Use `jq` externally for arbitrary output projection; no built-in query language.

## Safety and dry-run

No implicit prompts, even on a terminal. Create/update execute normally. Deletes, removals, revocations, destructive transfers/resets, archives with cascading effects, security/permission changes, bulk-delete and overwrite require `--yes` before **any** remote request or secret lookup. The catalog classifies each operation by actual side effect, not just HTTP verb. A body-driven action that may remove links requires `--yes` when its remove set is nonempty. Unknown destructive classification blocks that mapping's review, not safety enforcement at runtime.

`--dry-run` validates local input and emits the sanitized request plan without credentials, keyring access, OAuth refresh, remote lookups, presigned requests or mutation. It does not require `--yes`; reports `requires_confirmation` instead. No server permission/existence/feature claim. For uploads it shows local metadata and the planned phases, not fake signed URLs. A real destructive run still requires `--yes`.

## JSON results

Default stdout is exactly one compact JSON value plus newline. Do not vary by TTY. Success envelope:

```json
{"schema_version":1,"data":{"id":"opaque-id","name":"Example"},"meta":{"command":"work-item get","api_version":"v2","http_status":200,"status":"completed"}}
```

`data` preserves the complete version-native API response (including native list envelope and unknown fields), except operation-declared secret handling. HTTP 204 becomes `data:null` with `meta.http_status:204`; a JSON null response is distinguishable by status. `meta.command`, `status` are required; API commands include `api_version` and `http_status` when a response was received. Local commands omit API-only metadata. No timestamps or stored credentials by default; intentional one-time secrets follow [secret delivery](#secret-delivery).

One-page lists retain native bodies: v2 items are `.data.data`; typical paginated v1 items are `.data.results`. Native bare arrays stay arrays. `--all` deliberately returns `data:[<native page 1>,<native page 2>,...]` and `meta.pagination:{"mode":"all","pages":N,"complete":true}`. This preserves each page's metadata/unknown fields instead of inventing a lossy common resource schema. Leaf schemas document this opt-in shape. See [pagination](api.md#pagination).

Bound each ordinary JSON response to 16 MiB and complete aggregated output to 64 MiB. Aggregate via an owner-only temporary spool, not an unbounded in-memory slice. Validate all pages before writing stdout; on failure remove the spool and leave stdout empty. Limit errors must identify the limit and the manual pagination alternative, never claim complete results. Read errors normally produce no stdout. Disk/stdout writer failure is a local error; partial bytes already sent to a broken pipe cannot be recalled.

At the process boundary handle SIGPIPE so a closed stdout pipe becomes a checked write error, runs cleanup and uses this exit contract rather than Go's default SIGPIPE termination. Do not try to write a second JSON value to an already failed stdout. Report a safe stderr error if that stream remains usable.

Native bulk/multi-step outcomes can be partially successful: emit safe results with `meta.status:"partial"`, one structured error on stderr and exit 6. HTTP 202 means `meta.status:"accepted"`, not completed; preserve documented polling handle, never automatically rerun submission or wait indefinitely.

## Errors and exit codes

One compact JSON error line on stderr; optional warnings are separate structured lines shaped `{"schema_version":1,"warning":{"code":"http_connection","message":"HTTP does not encrypt traffic","command":"connection add"}}`. Warnings do not change exit status; HTTP warnings are setup-only as specified in authentication. Consent instructions are a deliberate auth-flow exception. stdout stays empty for failures unless the operation explicitly returns partial mutation outcomes. Default error shape:

```json
{"schema_version":1,"error":{"code":"api_error","message":"Request rejected","command":"work-item update","api_version":"v2","http_status":409,"upstream_code":"conflict","retryable":false}}
```

Only known safe normalized fields appear. Add safe field-level validation details, `retry_after_seconds`, and recovery IDs when meaningful. `retryable` means an explicit caller retry may be appropriate, not that the CLI retried; uncertain mutation outcomes use `outcome_unknown:true` and no blanket retry advice. Do not echo arbitrary response bodies, request bodies, HTML proxy responses, auth headers, signed URLs or secret-bearing validation values. Auth exchange errors suppress reflected server messages.

| Exit | Meaning |
| --- | --- |
| 0 | Completed, explicitly accepted, or pending headless login |
| 1 | Local/config/storage/output failure |
| 2 | Usage/input/schema/confirmation error |
| 3 | Authentication or authorization (401/403) |
| 4 | Transport/deadline/TLS failure |
| 5 | Upstream API/protocol failure, including 402/404/409/429 or malformed responses |
| 6 | Partial mutation, or known remote change followed by local delivery/commit failure |
| 7 | Operation unavailable in selected API version, known locally |
| 130 | Interrupt cancellation |

Unknown commands remain usage errors with concise suggestions. Do not turn upstream 404 into exit 7 or “unsupported edition”: it can mean hidden/missing data. New stable codes may be additive; incompatible envelope/flag/exit semantics require a major CLI version.

Error precedence: invalid local input/confirmation fails before side effects. Once any mutation phase is known to have succeeded, a later error (including cancellation, failed confirmation, storage or stdout failure) yields exit 6 with safe completed phases, cause and recovery IDs; do not lose that fact behind exit 1/4/130. Otherwise interrupt is 130, deadline/transport is 4, local storage/output/aggregate/page-limit is 1, and malformed/oversized upstream bodies or invalid continuations are 5. A mutation whose response was lost uses the underlying transport/protocol/interrupt code with `outcome_unknown:true`, not an invented successful phase. HTTP 202 is exit 0/accepted unless a later delivery step fails. When stdout is broken, partial metadata is carried in the stderr error only. No automatic retry or rollback in any case.

## Tables and files

Explicit `--output table` is supported for common list/get of projects, work items, states, labels, cycles, modules, milestones, members, pages, initiatives, releases and customers. Use deterministic columns declared in each descriptor (ID/identifier/name and relevant status), blank for absent fields; no extra relation fetches, terminal-width-dependent omission, color or control sequences. Strip terminal control characters from cell text, not JSON data. Reject table mode before requests for operations without a declared projection. Errors stay structured JSON on stderr. No table contract stability promise beyond documented columns; automation should use JSON.

Downloads require `--output-file PATH`; never send bytes to stdout by default. Use a secure sibling temporary file, stream, then atomically install without overwriting by default. `--overwrite --yes` explicitly permits replacement. Success stdout contains only safe local path/size/asset identity, not signed URLs. A failed transfer never leaves a seemingly complete destination.

## Secret delivery

An explicit create/rotate/credential-issuance or upload-phase operation whose purpose includes returning a secret emits its declared secret fields in the normal JSON result by default. No extra opt-in, prompt or file argument. Its help/schema must mark the result sensitive and show `--secret-output-file PATH` as the opt-in safer alternative for CI/logging environments. This exception never exposes stored PAT/OAuth credentials or secret fields from ordinary list/get operations; diagnostic and dry-run output remains redacted. Upload/download convenience commands consume signed material internally and return safe metadata only.

`--secret-output-file PATH` delivers the declared secret-bearing JSON payload into a 0600 owner-only file; stdout contains its path and redacted native data, never a second copy of the secret. Use a secure temporary sibling and atomic no-clobber installation. An existing destination fails locally unless `--overwrite --yes` explicitly permits replacement. Before any remote mutation, validate/open the temporary destination and check replacement policy; on failure send no request. Recheck safely at installation; a successful preflight cannot guarantee a later write/rename will succeed. Protected output must not silently fall back to stdout.

For default stdout delivery there is no portable preflight guarantee that a pipe will remain writable. If remote creation/rotation succeeds but output or protected-file installation fails, return exit 6, mark `remote_changed:true` and `secret_delivered:false`, and give available safe resource IDs and the documented recovery command. Remove incomplete temporary output; do not claim a lost one-time secret is recoverable, and do not automatically retry/rotate/revoke. If available, recovery is an explicit read or authorized rotation/revocation; otherwise explain that operator action is needed. This unavoidable failure window is documented, not “solved” by forcing every caller to use files.
