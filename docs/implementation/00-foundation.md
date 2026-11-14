# P00 — Tooling, discovery and coverage framework

Read [architecture](../design/architecture.md), [CLI contract](../design/cli-contract.md), [development](../development.md) and [API inventory](../design/api.md#source-inventory-and-coverage). No network/resource/auth implementation in this packet.

## Own

`go.mod`, `go.sum`, `cmd/plane-cli/main.go`, `internal/cli/{root,run,schema,version}.go`, `internal/output/`, `internal/spec/`, `internal/cmd/{checkapi,checkdocs}/`, `api/{catalog.json,provenance.json,sources/}`, `Makefile`, `.golangci.yml`, `tools/versions.mk`, `scripts/install-tools.sh`, `.github/workflows/check.yml`, relevant `.gitignore` entries. Create only packages used by working behavior; do not fill out the whole architecture with stubs.

## Ordered work

1. Initialize `github.com/foae/plane-cli`, pinned Go/Cobra and tools. Establish formatting, vet-equivalent lint, tests and module-cleanliness recipes; validate linter config. Set up the required CI baseline without hooks or publishing.
2. Implement fresh command construction and injected streams/context; one main exit boundary; offline root/group help, version, command/schema discovery, stable JSON errors. Demonstrate process exit behavior, not just returned Go errors. API metadata is not invented for local results.
3. Define the catalog/provenance schema from the API contract. Reconcile the complete TSV into a source ledger: resource owner, guide/overview/operation-source disposition, supplemental migration families and known evidence gaps. Do not guess method/path from slugs. Exact operation extraction may proceed per domain packet, but every source row already has an owner.
4. Review representative v1/v2 project/work-item operations and attachment/bulk/error shapes to prove metadata can express differences. Compile reviewed implemented descriptors offline; public discovery must not advertise planned, unimplemented commands as callable. Coverage checks distinguish documentation sources, operation mappings, implementation and scenario evidence.
5. Build checkers for stale metadata/local links and malformed provenance. Report missing coverage, do not mask it with blanket exclusions. Release mode requires full verified coverage; intermediate packet mode explicitly names only its completed domain scope and reports remaining scope.

## Acceptance

`make fmt-check`, `make lint`, `make test`, `make mod-check`, catalog/docs checks and host build are available. One offline binary run against an empty home proves help, version and schema work without credentials, keyring/network or config writes; unknown command/flag gives exit 2 and a single safe structured error. A large integer survives JSON output. Formatting check fails on deliberately unformatted temporary fixture/source then leaves it unchanged; remove the probe afterward.

Review catalog format, output/error/command conventions and tool/CI pins with the planner before P01. Do not claim no-op API checks prove full coverage. Add the working contributor setup guide now, not before recipes exist.
