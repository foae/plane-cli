# Offline API metadata

The [catalog](catalog.json) contains callable local/API command descriptors, operation contracts and the complete source ownership ledger. P00–P02 provide discovery, transport and authentication; P03 adds documented identity, project, state/label and work-item operations. `implemented` is distinct from planner-approved `verified` coverage. Discovery embeds only `schema_version`, `commands`, and `resources`; `operations` and `sources` are evidence, never callable registrations.

## Data model

The root `schema_version` is `1`. `commands` follow the [architecture descriptor contract](../docs/design/architecture.md#offline-metadata-and-validation): tokenized `path`, explicit Go `handler` ID, summary, side effect, examples, local classification, and version-indexed schema/flag variants. Flag bindings are JSON pointers into the same input schema. The CLI's registered handler IDs must match descriptors exactly.

`operations` are independently authored, deliberately minimal wire contracts. Each records its ID, owning packet, API generation, canonical command tokens/action/scope, exact method and slash-terminated path, request and response schemas, statuses/content type, pagination, OAuth scopes, feature evidence, side effect, sensitivity, version availability, provenance, lifecycle, and acceptance scenario IDs. `error_contract` links the reviewed v2 problem schema and its independently fetched provenance. `bulk` records native limits, atomic behavior and row indexes. Signed attachment target/form paths are sensitive; phase creation intentionally delivers them, ordinary reads redact them.

Unimplemented representative records remain `documented`, with `review_status` identifying partial contracts, **not implementation-ready** schemas. Implemented command descriptors carry operation-specific invocation validation; native responses remain extensible and are passed through without closed-shape response validation. Unknown sensitivity in extensions blocks domain implementation review. Operation-specific attachment identifiers do not establish a universal identifier policy. No universal edition compatibility is claimed.

Sensitivity must be an object with nonempty `review` and `extension_fields` strings, a boolean `intentional_secret_delivery`, and arrays of slash-prefixed path strings for `input`, `output`, `ordinary_read_redaction`, `signed_target_paths`, and `signed_form_paths`. Empty arrays explicitly declare no known paths; null or omitted fields do not constitute a review.

Every original TSV row appears once in `sources` with `id`, `version`, `family`, `packet`, `disposition`, `evidence_status`, and exact `url`. `family_owners` explicitly enumerates all 120 indexed families. Overviews are not operation counts. The six declared supplemental v1 evidence families have independent ownership even though absent from the TSV. Pending sources need no invented method or path. Reviewed sources link `provenance_id`; pending supplemental evidence can link migration provenance without claiming exact operation review.

Release completeness distinguishes operations from reviewed non-operation material. A `general-guide` or `overview` resolves without an invented API operation only with `evidence_status: "reviewed"`, `review_resolution: "non-operation"` and valid retained provenance. That explicit resolution means review found no unextracted methods; pending or merely partially reviewed overviews remain pending. An alias requires `disposition: "duplicate-alias"`, reviewed evidence, `review_resolution: "duplicate-alias"` and `alias_of` naming a resolved same-version source. Missing/pending targets and alias cycles never close coverage. Operation sources, supplements and evidence conflicts cannot use the non-operation exemption; they remain pending until their operations are verified or their evidence is explicitly reconciled into another appropriate disposition. P00 does not claim these resolutions for its pending rows.

Packet identifiers are exactly `P00` through `P10`. Packet mode requires a `packet_completion` declaration for its selected packet; release mode requires all eleven declarations. Each completed ID set must exactly equal that packet's verified operations, without duplicates or cross-packet IDs. P00 requires an empty set and `status: "metadata-only"`. Other packet statuses are nonempty descriptive review metadata, with future vocabulary defined by the planner, not a substitute for lifecycle/evidence checks.

The source index uses strict tab-delimited CSV quoting: fields containing literal quotes must use CSV quoting and doubled embedded quotes. Malformed quoting is rejected rather than silently reinterpreted.

## Provenance and retention

[provenance.json](provenance.json) pins the TSV bytes and records bounded official page fetches: exact URL, fetch date, upstream revision limitations, SHA-256 of fetched bytes, byte length, permission status, evidence kind/path and evidence hash. Documentation redistribution permission is unclear, so the fetched source text is **not** redistributed. [sources/](sources/) retains only minimal independently authored contract JSON, never an entire documentation site. The checker verifies retained evidence hashes offline; original source hashes are immutable fetch attestations and cannot be re-derived offline without the unretained source bytes. Refreshing a source is an explicit fetch/diff/review, never startup/build behavior.

A `verified` operation needs five existing repository files linked by `verification`: `operation`, `expected_fixtures`, `acceptance_command`, `acceptance_result`, and `planner_approval`. Paths must be repository-relative regular files; symlinks are allowed only when their resolved targets remain inside the repository. Handler presence, schema generation, or a status string is insufficient. The checker requires links but does not substitute for independent human approval of their contents.

## Development commands

From repository root:

```sh
go run ./internal/cmd/genspec
go run ./internal/cmd/checkapi --mode packet --packet P00
go run ./internal/cmd/checkapi --release
go run ./internal/cmd/checkdocs
```

`genspec --input PATH --output PATH` calls the shared deterministic generator. `checkapi` regenerates in a temporary directory, reads that generated file and compares exact committed bytes, then checks coverage, ownership, provenance/evidence hashes, offline schemas, duplicates, handler mapping and lifecycle evidence. Packet mode prints the explicit completed set and pending source counts by packet; release mode fails incomplete coverage. Release failure is expected at P00, not waived. `checkdocs` validates repository-local documentation links/fragments and embedded schemas without network access. Only [current usage examples](../docs/usage.md) are runnable examples; approved design documents intentionally describe future commands.

Implemented and verified operations must reference existing Go tests or statically discoverable subtests in `acceptance_scenario_ids`; unknown references fail `checkapi`. Documented, pending operations may retain planned scenario IDs, which do not constitute acceptance evidence.
