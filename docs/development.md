# Development and fast feedback

The P00 contributor recipes below are implemented. They do not install hooks or publish releases. Run commands from the repository root; choose a feedback depth locally, then run the reviewed packet gate.

## Setup and pins

Install the exact Go version in [`tools/versions.mk`](../tools/versions.mk) and put it on `PATH`. The initial toolchain is Go 1.27.4; the module's source compatibility floor is `go 1.27.0`. `GOTOOLCHAIN=local` makes a missing or different toolchain fail rather than silently downloading a replacement.

Prerequisites are GNU make, a POSIX shell, Go, and standard Unix tools. Explicit tool installation also needs `curl`, `tar`, `awk`, and either `sha256sum` (Linux) or `shasum` (macOS). Linux/macOS amd64/arm64 hosts are supported by the installer. Race tests additionally need a C compiler; GitHub-hosted Linux runners provide one.

```sh
make help
make tools
make check
make build
./bin/plane-cli --help
```

[`tools/versions.mk`](../tools/versions.mk) is the only authoritative development-tool pin file: Go, golangci-lint, govulncheck/x-vuln, and the four supported golangci-lint archive SHA-256 values live there. `scripts/install-tools.sh` parses literal values without sourcing or evaluating Make syntax. It downloads the pinned host archive over HTTPS, checks its SHA-256 against the committed reviewed value before extraction, installs golangci-lint into `.bin/`, installs the exact govulncheck module with `GOBIN` pointing at repository-local staging, and verifies the linter configuration. It does not replace global tools.

Checks never install or upgrade tools. `make tools` is the explicit networked setup/update step; `.bin/`, `bin/`, `dist/`, `.env` variants and `secrets/` are ignored. Tool installation may populate the external Go build/module cache, but its repository writes are confined to `.bin/`. Never commit real credentials.

Make and CI set `GOFLAGS=-mod=readonly`, `GOTOOLCHAIN=local` and `GOSUMDB=sum.golang.org`. Keep `go.mod` and `go.sum` committed. Module downloads and ordinary Go cache writes can still occur; “no writes” below means no repository source/module mutation. Download required modules before expecting entirely disconnected execution.

Runtime dependency pins belong in `go.mod`, not the tool pin file. The initial design baseline is Cobra v1.10.2 and `github.com/santhosh-tekuri/jsonschema/v6` v6.0.2 for P00; go-keyring v0.2.8, x/oauth2 v0.38.0 and x/sys v0.50.0 are later-packet baselines, not a reason to add unused dependencies. Add runtime dependencies only when used. The validator's [pinned module](https://github.com/santhosh-tekuri/jsonschema/blob/v6.0.2/go.mod) declares Go 1.21; exact-number handling and rejection of network schema loading remain consumer-contract requirements.

## Recipes

`make help` describes prerequisites, purpose and side effects. The Makefile is the shared local/CI interface, not a separate task-runner framework.

| Target | Behavior | Mutation/prerequisites |
| --- | --- | --- |
| `help` | List recipes and options | None; no Go execution |
| `tools` | Install verified pinned development tools; verify lint config | `.bin/`, external Go cache; network/setup tools |
| `fmt` | `go fmt ./...` | Go source |
| `fmt-check` | Fail on gofmt errors or differences under `cmd/` and `internal/` | None; temporary report removed |
| `vet` | `go vet ./...` | Go cache only |
| `test` | `go test $(PKG) $(TESTFLAGS)` | Test temp/cache |
| `lint` | Verify `.golangci.yml`, then `.bin/golangci-lint run ./...` | Cache only; `make tools` first |
| `mod-check` | `go mod tidy -diff` | No module writes; module cache/network possible |
| `api-check` | Offline operation/schema/provenance/coverage reconciliation for `PACKET` | None; default `PACKET=P00` |
| `docs-check` | Offline local links and examples/catalog consistency | None |
| `generate` | Regenerate committed embedded metadata from the reviewed catalog | `internal/spec/catalog.gen.json` |
| `check` | fmt-check, lint, test, mod-check, api-check, docs-check, version-check | No source writes; installed tools required |
| `race` | `CGO_ENABLED=1 go test -race ./...` | Test temp/cache; supported host and C compiler |
| `security` | `.bin/govulncheck ./...` reachable-advisory scan | Cache/network; installed tools required |
| `fuzz` | One named fuzz target in one explicit package for bounded seconds | Fuzz cache; explicit package/target required |
| `build` | Build `./cmd/plane-cli` into `bin/plane-cli` | Host build output |
| `cross` | CGO-free Linux/macOS amd64/arm64 builds | Four `bin/plane-cli-OS-ARCH` binaries |
| `change` | Create a uniquely named release-impact fragment | `changes/`; explicit `LEVEL` and `SUMMARY` |
| `version-plan` | Validate release inputs and report the pending bump/notes | Read-only |
| `release-prepare` | Update version/notes and consume fragments | Clean tree; explicit `VERSION`; no tags/publication |
| `version-check` | Validate version/history and optional PR release impact | Read-only; `BASE` enables change-set check |

`snapshot` is P10 work and is not implemented or represented by a no-op target. Archives/checksums and publishing are outside P00.

P10a provides SemVer change-entry, version-plan, release-preparation and check recipes. [Releases](releases.md#version-policy-and-tooling) defines the single version source, per-work-item release-impact policy and clean-tag build identity. No git hooks or automatic tag/publish step.

`PKG ?= ./...`, `TESTFLAGS ?=`, `PACKET ?= P00`. A focused local loop:

```sh
make test PKG=./internal/cli TESTFLAGS='-count=1'
make fmt
make lint
make check
```

Choose the relevant package/scenario and use `TESTFLAGS='-run TestName -count=1'` for an existing named test. Normal test caching is useful; do not disable it in every routine invocation. `check` includes `govet` through golangci-lint, so it does not repeat standalone `vet`. Do not create a second weakened definition of correctness.

`fmt-check` traverses the owned Go roots `cmd/` and `internal/` using `find -exec gofmt -l {} +`, captures the report in a temporary file and fails on either gofmt errors or nonempty output. It never formats source as a hidden prerequisite. If another Go source root is added, extend the traversal in the same change. Acceptance must exercise an unformatted temporary source and prove it remains unchanged after failure.

The process acceptance test builds from a temporary source-only copy (no repository catalog or generation step), then runs from an empty home. It exercises offline discovery, safe help errors and real closed stdout pipes.

### Metadata and coverage

```sh
make generate
make api-check
make api-check PACKET=P00
make docs-check
go run ./internal/cmd/checkapi --release
```

The checker entry points are `./internal/cmd/checkapi`, `./internal/cmd/checkdocs`, and `./internal/cmd/genspec`. Packet mode explicitly names the completed operation set and reports pending scope; passing P00 does not claim full API coverage. Release mode requires full verified coverage and is not the P00 gate. Generation is explicit and offline; build/install/startup use committed embedded metadata without repository-relative reads or generation. Review catalog/provenance and generated metadata changes together.

### Race, security and fuzzing

```sh
make race
make security
make cross
```

`security` preserves scanner diagnostics and fails on every nonzero exit. With the pinned scanner's default text output, exit 3 is reported distinctly as reachable vulnerabilities; other failures are an incomplete scanner/infrastructure gate, not a clean scan. Advisory-database outages must not be ignored or reclassified as success. See the pinned upstream [exit-code implementation](https://github.com/golang/vuln/blob/v1.9.0/internal/scan/errors.go).

For fuzzing, select a checked-in `FuzzName` and its single package, then run `make fuzz FUZZ_PKG=./internal/package FUZZ_TARGET=FuzzName FUZZTIME=30s`, substituting the actual existing package/target. The recipe rejects package patterns/lists, confirms the named target exists, anchors its name and requires a positive bounded duration in seconds. P00 does not claim future HTTP URL/redaction/body-parser/pagination fuzz targets already exist. Normal `make test` runs checked-in fuzz seeds; bounded mutation runs are separate from the fast gate. Never use unbounded CI fuzzing.

## Linter policy

[`.golangci.yml`](../.golangci.yml) uses the pinned binary's v2 schema, read-only module loading and test analysis. It enables only errcheck, govet, ineffassign, staticcheck, unused, bodyclose, contextcheck, nilerr, errorlint and nolintlint. Preset exclusions are empty and issue-count limits are disabled. `nolintlint` requires a specific rule and explanation.

Both `make tools` and `make lint` run `.bin/golangci-lint config verify`. Never enable all linters, blanket-exclude tests, use `--fix` in a check, add arbitrary complexity/line-length gates or broadly suppress findings. Add rules only for demonstrated useful signal; reachable dependency scanning is a separate concern.

## Continuous integration

[`.github/workflows/check.yml`](../.github/workflows/check.yml) runs these required push/PR lanes:

- Linux `make check`, including native unit/CLI tests.
- Native macOS `make test`; Linux tests are not redundantly duplicated in another ordinary-test job.
- Linux `make race` with the runner's C toolchain.
- `make security` with the public advisory database; no Plane credentials.
- `make cross` for all four CGO-free Linux/macOS amd64/arm64 targets.

The workflow reads Go from the same literal pins through `sh scripts/install-tools.sh --print-go-version`. It grants read-only contents permission, does not persist checkout credentials, pins actions to full upstream commit SHAs and caches Go data by OS/architecture/Go plus `go.sum` and `tools/versions.mk`. Tool installation is explicit in the lint/security lanes. It neither publishes artifacts/releases nor installs hooks. Fork PRs must never need real Plane secrets. Scheduled dependency updates and bounded fuzz mutation jobs are separate future decisions, not hidden parts of the fast gate.

Reviewed upstream pin evidence:

- [golangci-lint release checksums](https://github.com/golangci/golangci-lint/releases/download/v2.15.0/golangci-lint-2.15.0-checksums.txt), committed as the four SHA-256 assignments in `tools/versions.mk`.
- [checkout v6.0.2 ref](https://api.github.com/repos/actions/checkout/git/ref/tags/v6.0.2): `de0fac2e4500dabe0009e67214ff5f5447ce83dd`.
- [setup-go v6.3.0 ref](https://api.github.com/repos/actions/setup-go/git/ref/tags/v6.3.0): `4b73464bb391d4059bd26b0524d20df3927bd417`.

## Update and documentation discipline

Read upstream migration notes before tool/schema updates; update the single authoritative pin/catalog, review new upstream checksums/revisions, then run configuration validation and the affected gate. Do not bundle unrelated dependency updates or use `@latest` in CI. Changes to CLI/schema/auth/version semantics need planner review before broad resource work.

Update a feature's task guide/generated reference in the same packet. Keep actual dated run evidence in a bounded packet report, not appended forever to this guide. Available recipes are not evidence that checks have passed. See [verification](verification.md).
