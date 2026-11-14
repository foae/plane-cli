# Development and fast feedback

These are implementation requirements for P00/P10, **not commands available in the repository yet**. No git hooks of any kind. Contributors choose a feedback depth; CI enforces the reviewed gate.

## Toolchain and dependency baseline

Pin Go 1.27.4 for local recipes/CI; `go 1.27.0` is the source compatibility floor. Use `GOTOOLCHAIN=local` in reproducible CI so a missing toolchain fails rather than silently downloading another. Distribute binaries so users do not need Go. Commit go.mod/go.sum, use `GOFLAGS=-mod=readonly` for checks and retain module checksum verification.

| Component | Initial pin | Purpose |
| --- | --- | --- |
| Go | 1.27.4 | Build/test/gofmt/vet |
| golangci-lint | 2.15.0 | Explicit v2 linter set |
| govulncheck / x/vuln | v1.9.0 | Reachable advisory scan |
| Cobra | v1.10.2 | CLI parsing/help |
| go-keyring | v0.2.8 | Linux/macOS secret backend |
| x/oauth2 | v0.38.0 | Compatible standard OAuth exchange |
| x/sys | v0.50.0 | Unix locking/secure IO |
| JSON Schema validator | `github.com/santhosh-tekuri/jsonschema/v6` v6.0.2 | Offline 2020-12 input validation |

These versions exist in the researched sources; P00/P02 must verify their assembled Go requirements and platform behavior. Add runtime dependencies only in the packet that uses them. Do not add a module merely to match this table. New dependency/tool versions require a focused review, not `@latest` in CI.

The validator's [pinned module](https://github.com/santhosh-tekuri/jsonschema/blob/v6.0.2/go.mod) declares Go 1.21. P00 must prove exact-number validation and reject network schema loading, not assume library defaults meet the CLI contract.

Store development tool pins in `tools/versions.mk`; runtime pins remain in go.mod. A small `scripts/install-tools.sh` used by `make tools` installs verified pinned golangci-lint release archives for the host into `.bin/`, plus `GOBIN=<repo>/.bin go install golang.org/x/vuln/cmd/govulncheck@v1.9.0`. Verify downloaded archive checksum against reviewed release checksum data. No `curl | sh`, implicit global replacement or build-time network schema fetch. Tool setup is explicit; normal check targets do not install/upgrade tools. `.bin/`, build output and local secrets are ignored.

## Recipes

`make help` lists target purpose, side effects and prerequisites. Keep a plain Makefile, not a second orchestration framework. CI calls these same targets. The planned recipes are:

| Target | Behavior | Mutation |
| --- | --- | --- |
| `tools` | Install exact development tools locally | `.bin/` only |
| `fmt` | `go fmt ./...` | Go source |
| `fmt-check` | Fail for gofmt differences | None |
| `vet` | `go vet ./...` | None |
| `test` | `go test $(PKG) $(TESTFLAGS)` | Test temp/cache only |
| `lint` | `.bin/golangci-lint run ./...` | None |
| `mod-check` | `go mod tidy -diff` | None |
| `api-check` | Offline operation/schema/coverage reconciliation | None |
| `docs-check` | Local links, examples/catalog consistency | None |
| `check` | fmt-check, lint, test, mod-check, api-check, docs-check | None |
| `race` | `go test -race ./...` | Test temp/cache only |
| `security` | `.bin/govulncheck ./...` | Cache/network |
| `fuzz` | One named target/package, bounded time | Fuzz cache |
| `build` | Host binary in `bin/plane-cli` | Build output |
| `cross` | Four CGO-free target binaries | Build output |
| `snapshot` | Versioned archives/checksums, no publishing | Release output |

`PKG ?= ./...`, `TESTFLAGS ?=`. Fast local loop:

```sh
make test PKG=./internal/httpx TESTFLAGS='-run TestCredentialIsolation -count=1'
make fmt
make lint
```

Choose the relevant package/scenario, then run the packet gate once after the change. `go test` caching is useful; use `-count=1` for deliberate uncached scenario proof, not every routine invocation. `check` includes `govet` through golangci-lint; standalone `make vet` is available but not repeated in that gate. Do not make “fast” a second weakened definition of correctness.

A correct nonmutating formatting recipe must fail on either gofmt errors or listed differences. `gofmt -l` alone exits zero for unformatted files. Suggested POSIX-shell body (double `$` in Makefile):

```sh
set -eu
report=$(mktemp)
trap 'rm -f "$report"' EXIT HUP INT TERM
find cmd internal -type f -name '*.go' -exec gofmt -l {} + > "$report"
if [ -s "$report" ]; then
  cat "$report"
  exit 1
fi
```

P00 owns the source-root set; if another Go source directory is added, extend this traversal in the same change. `find -exec ... +` handles spaces and propagates gofmt failures. `fmt` is an explicit writer, never a hidden prerequisite of CI. Do not implement formatting checks by formatting first and then declaring success.

## Linter configuration

Initial `.golangci.yml` policy, validated against the pinned v2 binary in P00:

```yaml
version: "2"
run:
  timeout: 5m
  modules-download-mode: readonly
  tests: true
linters:
  default: none
  enable:
    - errcheck
    - govet
    - ineffassign
    - staticcheck
    - unused
    - bodyclose
    - contextcheck
    - nilerr
    - errorlint
    - nolintlint
  settings:
    nolintlint:
      require-explanation: true
      require-specific: true
  exclusions:
    presets: []
issues:
  max-issues-per-linter: 0
  max-same-issues: 0
```

Use `.bin/golangci-lint config verify` during setup/upgrades. Never `enable-all`, blanket test exclusions, `--fix` in a check, arbitrary complexity/line-length gates or broad nolint suppression. A targeted suppression requires a specific rule and explanation. Add more security/style linters only for demonstrated useful signal; reachable dependency scanning is a separate concern.

## CI plan

`.github/workflows/check.yml`: pinned checkout/setup actions by full commit SHA, read-only permissions, Go toolchain and tool versions from the same reviewed sources, cache keyed by OS/Go/go.sum/tool versions. Required PR jobs: `make check` on Linux; native Linux/macOS unit/CLI fake-server tests; race detector on a supported runner with a C toolchain; `make security`; four-target cross-build. Avoid repeating identical tests in multiple jobs without a distinct purpose. Never use real Plane secrets for PR jobs, especially forks.

Security scanner database unavailability is a failed/incomplete gate, not a clean scan. Report infrastructure versus vulnerability failure distinctly; do not silently ignore either. Scheduled updates and bounded fuzz runs are separate from the fast local loop. Pin GitHub action revisions and artifact checksums at implementation time using actual upstream evidence, not fabricated hashes in this plan.

Fuzz only boundary targets with useful invariants: URL path construction, redaction, body parser and pagination continuation. Example: `go test ./internal/httpx -run '^$' -fuzz '^FuzzURL$' -fuzztime=30s`. Require one explicit package/target; never unbounded CI fuzzing. Run checked-in fuzz seeds in normal tests.

## Update and documentation discipline

On tool/schema updates: read upstream migration notes, update the one authoritative pin/catalog, run config validation and the affected gate, update focused docs. Do not bundle unrelated dependency updates. Changes to CLI/schema/auth/version semantics need planner review before broad resource work.

Once a feature exists, update its task guide/generated reference in the same packet. Keep dated run evidence in a bounded packet report, not appended forever to a giant contract. Do not change current documentation to claim a test you did not run. See [verification](verification.md).
