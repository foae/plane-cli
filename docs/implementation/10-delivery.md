# P10 — Full-scope reconciliation and delivery

Prerequisites P00–P09. Read [verification](../verification.md), [releases](../releases.md), [development](../development.md), [decisions](../planning/decisions.md).

## Own

Final catalog/provenance integration, documentation router/status, `scripts/` release packaging, `.github/workflows/release.yml`, `Formula/plane-cli.rb`, release metadata/checksum generation and focused artifact acceptance. Do not mutate external taps, tags, releases or live Plane workspaces.

## Ordered work

1. Reconcile all 987 source-index rows and supplemental families against reviewed operation records. Explain guide/alias dispositions; count actual unique operations separately. Every in-scope operation must have implementation and meaningful fixture evidence. No percentage headline hides an unfinished family.
2. Run the full agreed offline/mock acceptance gate, race/security and target builds. Inspect raw failures; do not relabel waived checks as passing. Review secret/input/redirect boundaries and the dependency/license set with the planner.
3. Build four archives/checksums/version metadata, exercise isolated host installation and `go install`, and validate/test the Homebrew formula against controlled assets. Exact publish destinations and action/checksum pins are resolved from actual authorized infrastructure, not guessed.
4. Review progressive disclosure: short root guidance, task router, focused working recipes and generated per-group reference. Remove stale planned claims only for behavior actually delivered; preserve historical independent design/decision records as history. Local links/examples/catalog pass checks.
5. Produce a final review report separating documented/mock-tested behavior, native platform evidence and unverified live edition compatibility. Record residual evidence limitations without implying unsupported deployment features are CLI defects.

## Acceptance and release decision

Planner verifies the final gate in [verification](../verification.md#final-gate). A complete local snapshot is deliverable without publishing. A full release cannot be called complete if required operation mappings are unresolved. If the maintainer authorizes publication, follow the reviewed release workflow; authorization for implementation is not permission to push tags or release artifacts.
