# API coverage and adapters

Approved design. Sources and confidence are recorded in [research](../reference/research.md). The target is full **documented public** REST coverage, separately mapped for v1 and v2—not MCP tool parity and not reverse-engineering private UI endpoints.

## Source inventory and coverage

[api-index.tsv](../reference/api-index.tsv) snapshots the official documentation index: 987 unique links, 649 v2 and 338 v1. Rows include introductions/overviews; they are **not an operation count**, verified schemas, or implementation status. Load a bounded slice:

```sh
awk -F '	' 'NR == 1 || ($1 == "v2" && $2 == "work-items")' docs/reference/api-index.tsv
awk -F '	' 'NR == 1 || ($1 == "v1" && $2 == "issue")' docs/reference/api-index.tsv
```

P00 establishes `api/catalog.json`'s format and the complete source ownership ledger; domain packets populate exact operation records before implementation. Every source row is accounted for as an operation source, general guide, duplicate/alias or evidence conflict, not a guessed URL slug. An overview may describe additional operations not separately indexed. Supplemental v1 families mentioned by migration must be assigned at P00 even when absent from the TSV: AI agents/runs/activities and integration jobs to P09, server upload variants to P06, core work-item searches/grouped counts/lite lists to P03, scheduling/collaboration equivalents to P04, approvals/state transfers to P09. The owning packet resolves the evidence before it can close; do not wait for P09 to discover a P03/P06 prerequisite.

Each operation record requires: stable local ID; one owning packet; version; canonical command-path tokens/action/scope; exact method/path including trailing slash; path/query/body types and presence/null rules; response/status/content types and pagination kind; scope/feature evidence; side-effect classification; source URL + snapshot hash/provenance; lifecycle state (`documented`, `implemented`, `verified`); acceptance scenario IDs. Sensitivity metadata identifies input and output paths (including nested arrays/maps), ordinary-read redaction, intentional secret-delivery fields, and signed target/form fields. Unknown sensitivity on a potentially secret-bearing operation blocks its review; never invent a safe default. Schema snapshots and manual supplements remain distinguishable.

`checkapi` must reconcile index coverage and supplements, duplicate routes/command mappings, referenced schema validity, actual command implementations and observable operation evidence. A stub, a generic HTTP escape hatch, help registration, or generated schema alone never advances an operation to verified. Aliases and shared routes do not inflate counts. Source refresh is an explicit diff/review, never build/startup network activity.

Provenance records the source revision/date, hash and permission/license for retained excerpts or snapshots; do not vendor entire documentation sites without checking reuse terms. Where redistribution is unclear, retain URL/hash and a minimal independently written contract rather than copying source text. A `verified` transition requires linked operation evidence, independently reviewed expected fixtures, acceptance command/result and planner approval; `checkapi` rejects missing links or self-declared status without these records.

## Version and edition policy

Explicit `v1|v2` belongs to a connection and may be explicitly overridden per invocation. No `auto` mode, fallback, route probing on failure, automatic migration or silently combined calls across generations. For a locally known v1-only command invoked as v2, fail before credentials or HTTP with exit 7 and actionable explicit-version guidance; the reviewed command union stays registered.

Cloud, Community and Commercial expose different releases/entitlements. Workspace feature flags describe switches/type governance, not a universal compatibility matrix or permission grant. An unrecognized flag is unknown; 402 means unavailable entitlement/feature, 403 authorization, 404 missing/hidden/possibly unavailable—not proof of edition. No minimum version or “fully compatible” statement without evidence. `workspace-feature get` and explicit read-only diagnostics may show facts, but ordinary CRUD does not preflight unrelated endpoints or cache inferred capability conclusions.

V1 may use nested relations, `results` pagination, legacy route names and nonuniform errors. V2 has sparse relations/ID fields, explicit pagination discriminators, problem+json, PATCH updates, and renamed fields. Keep wire DTOs and tests distinct. Do not fill absent response fields with null, rename server fields in `data`, or translate an input from one generation automatically.

## Resource work packages

Every method/action described by the source rows is in scope; labels below are grouping, not a CRUD-only restriction.

| Packet | Resource families | Special obligations |
| --- | --- | --- |
| P03 | Identity, projects, all state/label definitions and assignments, work items | Complete first lifecycle and supplemental searches |
| P04 | Cycles, modules, milestones, estimates/points, intake | Transfers, archives, summaries |
| P04 | Comments, links/relations, activities, worklogs, subscribers/votes, recurring items | Native actions and bounds |
| P05 | Project/workspace types, properties/options/contexts/type links | Schema, custom fields, epics |
| P06 | Project/workspace pages, page links, attachments, collections, published pages, stickies, assets | Transfers and content |
| P07 | Initiatives/labels, releases/tags/labels/comments/links/changelogs/project releases | Membership and scopes |
| P07 | Customers/requests/properties/values, teamspaces, views | Related records and scope |
| P07 | Work-item/page/project templates and categories | Apply/copy semantics |
| P08 | Members, invitations, roles, permissions/grants/schemes, group sync | Privilege changes |
| P08 | Project/workspace features, audit logs, user administration/notifications/artifacts, webhooks/logs | Sensitive responses; no state/label definitions |
| P09 | Project/workspace automations and workflow states/transitions/activities/hooks/type governance | Conflicts, graph/state actions |
| P09 | V1 agents/runs/activities, integrations/importers, approvals/state transfer | Supplemental evidence |

No separate invented `epic create` API: use work-item type/scope semantics where documented; preserve v1 epic-specific public operations where they actually exist. Do not assume a user-created type named “Epic” can acquire read-only `is_epic` semantics. The Plane MCP tool contract is not the REST contract.

## Pagination

Default one page, preserving the server envelope. Flags reflect the selected API: `--per-page`, `--offset`, `--cursor`, `--paginate` only where supported. `--all` follows native continuation without changing filters, fields, expansion or ordering. `--max-pages` defaults to 1000; exhaustion with more data is an explicit incomplete error, not successful truncation. CLI total deadline defaults to 5 minutes for an API invocation, per-request deadline 30 seconds; either is configurable. Interrupt cancels outstanding IO and spool writes.

Timeout values must be positive durations. Ordinary API/OAuth requests use the per-request timeout within the invocation deadline. Streaming storage requests use the remaining total deadline instead of the ordinary 30-second full-body timeout; bytes are streamed, not limited by JSON response caps. Enforce documented transfer size limits and known declared lengths, with no invented universal file-size ceiling. A user can explicitly raise `--timeout` for large files. OAuth consent waiting has a separate 10-minute default via `--login-timeout`; it may be shortened, not extended beyond the pending-state expiry. Token exchange still uses the request timeout within that login deadline. Headless completion cannot extend the recorded expiry.

V2: branch on `pagination.style`. Offset `next` is an integer, **not a URL**; `null` terminates. Per-page default 50/max 200; offset max 10000. Do not infer completion from short/empty data or compute continuation from array length. Cursor mode uses opaque `next_cursor` with `has_more`; preserve all noncursor parameters. Repeated/nonprogressing continuation, missing cursor with has_more, unknown pagination style or malformed continuation is a protocol error. Do not reject an empty page with a valid advancing cursor. Offset traversal reaching upstream limits reports the limitation; never silently switch to cursor/reorder. Operators can explicitly select cursor and eligible ordering for deep traversal; not every default ordering supports it.

V1: each endpoint declares its actual paginator/bare-array contract. Honor documented next cursor and next-page flags, not v2 assumptions. A nonpaginated array is one complete response unless evidence says otherwise. Never fetch a server-returned arbitrary URL with credentials; construct subsequent requests against the configured base from validated continuation data.

`--all` is an explicit multi-request snapshotless traversal: concurrent writes can affect results. Do not promise transactional exports, deduplicate records silently or conceal duplicates. Aggregate native pages as defined in [CLI output](cli-contract.md#json-results), stage before stdout, enforce body/total/page/deadline limits, and return safe failure progress metadata without partial read results.

## Fields, filters and types

V2 sparse lists may omit `description_html` and custom fields; fetching detail is explicit. `--fields` invalid names are upstream errors, not an excuse to strip fields. Preserve user-supplied HTML and editor-managed metadata; no invented Markdown converter or block-document API.

For v2 project writes, preserve the operation-specific input contract: `network` is a string even though project responses and list filters use integers. `start_date` and `target_date` accept `YYYY-MM-DD` per the explicit field prose, rather than the conflicting `date-time` type label. This interpretation was approved during P03 contract review; do not coerce input to match response types. Sources: [create](https://developers.plane.so/api-reference/v2/projects/create-project.md) and [update](https://developers.plane.so/api-reference/v2/projects/update-project.md).

For v1 work-item create/update validation, use the operation parameter definitions: `estimate_point` is a string and HTML content uses `description_html`. The overview's integer/null estimate type and examples' unlisted `description` field conflict with those definitions; neither expands the accepted write schema. Sources: [create](https://developers.plane.so/api-reference/issue/add-issue.md) and [update](https://developers.plane.so/api-reference/issue/update-issue-detail.md).

Exact REST filters use documented names/operators and AND semantics. Do not send PQL strings as REST filters because the MCP server accepts them. V2 identity/path behavior can differ per operation: the attachment page permits project/work-item identifiers while migration prose says paths take IDs. Prefer the exact operation's evidence, record the conflict, and test the permitted syntax—never generalize across all endpoints.

Type/project/workspace mode writes use the explicitly selected surface; a 409 governance conflict is actionable, not an instruction to retry on another route. Provide explicit feature/type-schema reads so agents can construct valid custom fields. No hidden workspace feature mutation to make a request succeed.

## Bulk and multi-step writes

Use native bulk endpoints where documented, including their max batch sizes (often 50) and `all_or_none` behavior. Validate per-operation bounds before sending. Never silently chunk oversized input into several nontransactional requests, emulate unavailable v1 bulk calls, or assume an upsert is safe to replay. Mixed per-item results yield partial status/exit 6 with stable input index/identity and errors. An all-or-none failure remains upstream failure; fixtures must prove actual documented response shape rather than inventing uniform bulk semantics.

There are no automatic HTTP retries, including 429/5xx. Parse safe Retry-After seconds/date and expose backoff guidance, respecting the configured deadline. Documented 60/minute API-key guidance is not a client-global hardcoded entitlement; server configuration/token classes differ. Avoid gratuitous lookups and concurrency. A timeout after sending a mutation may leave its outcome unknown: report it and suggest a safe explicit read, not an automatic retry.

HTTP 202 is accepted, not completed. Polling/get-status commands are separate. Multi-step failure reports completed phase IDs without secrets; never claim rollback or automatically delete uploaded assets that may be in use.

## Files, pages and secrets

V2 work-item attachment: metadata POST returns **200** with `upload_data`, `asset_id`, `asset_url`, attachment; stream multipart signed fields + file to storage; PATCH confirmation only after successful upload. Implement `attachment upload` convenience and all documented phase operations with [intentional secret delivery](cli-contract.md#secret-delivery). Workspace-page assets and v1 server-mediated variants need their own evidence; do not copy work-item shape blindly.

Storage uses a separate credential-free client and the exact [destination policy](authentication.md#transport-isolation): trust documented HTTP/HTTPS targets from the selected Plane server, including private hosts and HTTP targets returned by HTTPS APIs. No origin allowlist or per-transfer permission flags/warnings. Never forward API cookies/auth. Signed URL data remains sensitive; only intentional phase-command output may expose it. Failed confirmation is partial, with safe asset ID and recovery phase, not a successful attachment.

Downloads intercept documented redirects and stream into a secure temporary sibling before atomic installation. Do not use Content-Disposition as an unchecked local path. Verify known size/checksum when provided; remove partial temp files on error/cancel.

Page archive can archive descendants/unpublish; deletion may require prior archive. Expose explicit steps and confirmation; never secretly archive to make delete work. Project pages and workspace wiki/collections are separate. Published-page reads have their own auth/public rules. Schema/field/response mismatches must not be papered over by returning empty arrays or placeholders.
