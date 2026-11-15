# Scope and decisions

Planning record, 2026-11-14. User decisions are binding; engineering defaults below are explicit, reviewable choices.

## User decisions

- Full public REST API coverage: Cloud, Community self-hosted, Commercial self-hosted. Unsupported server features must be reported accurately, not fabricated or emulated.
- CLI only, implemented in Go; binary `plane-cli`. No MCP server.
- Explicit API v1/v2 selection per connection, JSON by default, explicit destructive confirmation.
- Linux/macOS, amd64/arm64. Release archives/checksums, `go install`, maintained Homebrew formula.
- PAT and full OAuth login/refresh, operator-owned app credentials. Environment, OS keyring and owner-only file backends must all be available and configurable.
- CLI-managed OAuth login saves credentials in owner-only files by default and renews automatically; OS keyring is opt-in. Environment token overrides remain externally managed for that invocation.
- The selected invocation destination authorizes sending environment credentials there; no additional destination-binding declaration. Stored credentials retain their explicit connection/base binding.
- Intentionally requested one-time secrets appear in JSON stdout by default. Protected-file delivery is opt-in and discoverable; logs, errors, dry-run and ordinary reads remain redacted.
- Protocol and destination are operator choices. Accept HTTP and HTTPS Plane/storage endpoints, including LAN/private hosts and HTTP storage returned by an HTTPS API, without extra permission flags or per-operation warnings. At most one HTTP warning at connection setup; no recurring nagging. Trust documented storage targets supplied by the selected Plane server.
- Explicit support for network-level credential injection: requests may run without locally established auth.
- Common field flags plus complete JSON-file/stdin input; bounded one-page lists, explicit `--all`; native bulk actions.
- Explicit `--output table` for common list/get operations, never terminal-dependent output switching.
- First working slice: connection/auth → projects → states/labels → work-item lifecycle. Broader coverage remains part of the full release.
- Official docs + mocks/fake HTTP servers are sufficient acceptance. No live end-to-end requirement. A Commercial Free instance is available later, not part of this task or a gate.
- Agent-first discovery and feedback; progressive-disclosure docs; no git hooks.
- Research and interview precede implementation. Record an independent design before examining Kaneo. Lower-capability implementers get bounded instructions; planner retains contract decisions and final review.
- First full stable CLI tag `v1.0.0`; subsequent work records SemVer impact through release tooling. Versioned binary and matching agent-skill artifacts share a release; GitHub Releases and a maintainer-selected Homebrew tap are distribution targets. See [release plan](../releases.md).
- A portable progressively disclosed agent skill includes a working P00–P03 core and indexes focused references for additional implemented families. Explicit CLI update checking is supported; automatic checking and self-installation are not. See [skill plan](../design/agent-skill.md).

## Engineering defaults

- Cobra, standard-library HTTP/JSON, internal-only packages; no runtime schema downloads or generated public SDK.
- Stable CLI envelope around version-native response bodies; schema version independent of REST version.
- Named `connection` commands. Configuration namespace `foae-plane-cli`, environment prefix `PLANE_CLI_`; avoid Plane Compose's binary/config/environment collisions.
- New connections require explicit base URL and API version. No implicit Cloud destination and no automatic version detection. A command may override version explicitly, never silently.
- Auth mode and storage are distinct. Default auth mode `api-key`, default persistent backend `file`; `env` and `keyring` remain explicit alternatives. No automatic persistence of environment overrides and no silent backend fallback.
- No general-purpose raw HTTP escape hatch or PQL translator in this release. Complete reviewed commands, not `api request`, satisfy coverage.
- No automatic HTTP retries, telemetry, background/startup network update checks, persistent capability cache, or auto-browser launch. Explicit `update check` is planned under [P10](../implementation/10-delivery.md) with no Plane credential access. OAuth renewal happens before API dispatch when expiry is known, never by replaying a failed mutation.
- HTTPS certificate verification remains enabled, with custom CA support rather than `--insecure`. Never silently rewrite a selected URL's scheme. HTTP acceptance is independent of TLS certificate validation.
- No generic batch orchestration or client-emulated bulk operations. No automatic rollback across several upstream calls.
- Go 1.27 baseline, reproducible checks with Go 1.27.4; tool/library pins in [development](../development.md). Maintain latest supported patch through reviewed updates, not floating CI downloads.

## Non-goals

Windows, a hosted OAuth broker, public multi-tenant OAuth registration, TUI, interactive setup wizard, daemon, MCP bridge, synchronization engine, shell installer, container/Linux-package distribution, undocumented private web-app endpoints, and migrations that silently reinterpret v1 payloads as v2.

## Remaining uncertainty, not delegated product decisions

Minimum upstream versions for v2 by edition are not established. Live schema availability and OAuth redirect registration acceptance vary. Public-client PKCE/device-code support was not established. The API index is incomplete relative to the migration page's v1-only list. These become explicit evidence tasks in P00/P02, not invented compatibility promises. See [research](../reference/research.md).

There are no unanswered product choices from the interviews. If research contradicts the approved contract materially, the implementer presents evidence and asks the planner; it must not silently narrow scope.
