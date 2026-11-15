# P10 — Full-scope reconciliation and delivery

Final release prerequisites: P00–P09. Preparation slices below may begin earlier at their stated prerequisites; they do not waive full-release coverage. Read [verification](../verification.md), [releases](../releases.md), [agent skill](../design/agent-skill.md), [development](../development.md), [decisions](../planning/decisions.md).

## Own

Final catalog/provenance integration, documentation router/status, version/release-note tooling, portable agent skill, explicit CLI update checking, `scripts/` release packaging, `.github/workflows/release.yml`, `Formula/plane-cli.rb`, release metadata/checksum generation and focused artifact acceptance. Do not mutate external taps, tags, releases or live Plane workspaces.

## Bounded delivery slices

P10a provides version tooling; remaining slices are implementation plans, not completed features. The canonical contracts above define behavior; create exact file/fixture handoffs before execution.

| Slice | Deliverable | Prerequisites |
| --- | --- | --- |
| P10a | SemVer change fragments, version planning/preparation/check tooling | P00 tooling and version metadata |
| P10b | Portable progressive-disclosure skill, P00–P03 core and supported family references | P10a version contract, reviewed P03 recipes |
| P10c | Four binary archives, skill archive, GitHub release workflow and Homebrew formula | P10a, P10b; full gate before stable publication |
| P10d | Explicit `update check`, offline discovery and skill recipe | P10a version comparison, P10c release metadata contract |
| P10e | Full reconciliation, integrated acceptance and v1.0.0 decision | P00–P09 and P10a–P10d |

Use P10a's release-impact fragments for each subsequent work item. P10b can grow with completed packets instead of waiting for all API families. P10c can test snapshots before coverage is complete. P10d uses fake GitHub responses and does not need a live release. P10e alone authorizes the technical stable-release recommendation; actual tag/publish/tap mutations still require maintainer authorization.

Acceptance by slice: P10a temporary-repository version tooling scenarios; P10b packaged skill recipes and agent disclosure review; P10c staged artifact/installation/formula checks; P10d fake-server update scenarios and zero-network ordinary invocation proof. Each updates focused documentation and reports exact checks, limitations and planner decisions. Do not count a passing cross-build as published binaries or a planning document as an installed skill.

## Ordered work

1. Reconcile all 987 source-index rows and supplemental families against reviewed operation records. Explain guide/alias dispositions; count actual unique operations separately. Every in-scope operation must have implementation and meaningful fixture evidence. No percentage headline hides an unfinished family.
2. Run the full agreed offline/mock acceptance gate, race/security and target builds. Inspect raw failures; do not relabel waived checks as passing. Review secret/input/redirect boundaries and the dependency/license set with the planner.
3. Build four archives/checksums/version metadata, exercise isolated host installation and `go install`, and validate/test the Homebrew formula against controlled assets. Exact publish destinations and action/checksum pins are resolved from actual authorized infrastructure, not guessed.
4. Review progressive disclosure per the agent-skill contract: short root guidance, P00–P03 working core, task router, focused working recipes and generated per-group reference. Validate the versioned skill archive with the matching binary. Remove stale planned claims only for behavior actually delivered; preserve historical independent design/decision records as history. Local links/examples/catalog pass checks.
5. Verify SemVer/version tooling, tag/metadata agreement and update checking against the release contract. Produce a final review report separating documented/mock-tested behavior, native platform evidence and unverified live edition compatibility. Record residual evidence limitations without implying unsupported deployment features are CLI defects.

## Acceptance and release decision

Planner verifies the final gate in [verification](../verification.md#final-gate). A complete local snapshot is deliverable without publishing. A full release cannot be called complete if required operation mappings are unresolved. If the maintainer authorizes publication, follow the reviewed release workflow; authorization for implementation is not permission to push tags or release artifacts.
