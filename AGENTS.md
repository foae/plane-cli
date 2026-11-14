# Plane CLI: agent entry point

This repository currently contains an implementation plan, not a working CLI. Do not report planned commands or recipes as implemented. Binary: `plane-cli`; module: `github.com/foae/plane-cli`.

## Read only what this task needs

1. Read [docs/README.md](docs/README.md) for the task router.
2. Read your assigned [implementation packet](docs/implementation/README.md) and its named contracts.
3. Load only that packet's API source rows. Never preload the full inventory or every document into context.

**Progressive disclosure is mandatory.** Keep this file short. Put enduring rules in one focused document; link instead of duplicating. Root help → command-group help → leaf help/schema follows the same rule. Distinguish current implementation, approved design, documented upstream behavior, and unverified assumptions.

## Non-negotiable engineering rules

- Idiomatic Go; standard library first, small packages, explicit errors/context, no speculative SDK or generic framework. Follow [architecture](docs/design/architecture.md).
- JSON-first, noninteractive by default. Never change stdout shape, exit codes, API-version semantics, destructive safeguards or auth policy without planner review. See [CLI contract](docs/design/cli-contract.md).
- Support explicit v1/v2 connections, all documented public operations, Linux/macOS amd64/arm64. No silent version fallback or undocumented private UI APIs.
- Credentials: environment, explicitly selected OS keyring or owner-only file; external-injection mode must work without local credentials. TLS stays verified. Never log secrets or forward API auth to storage. See [authentication](docs/design/authentication.md).
- No git hooks: do not create, install, suggest or configure them. Use [feedback recipes](docs/development.md); CI must run the same checks.
- Acceptance uses deterministic fake HTTP servers and controlled backend doubles, not a live Plane instance. Do not label mocks end-to-end or claim edition compatibility they cannot prove.
- Read the operation's official contract before implementing. Do not guess method/path/schema from a title or another API generation. Escalate contradictory sources with exact evidence.
- Make one bounded packet coherent end to end: implementation, observable acceptance, offline help/schema, inventory status, and focused docs. No stubs counted as coverage.
- Record the exact checks actually run. Review security and shared-contract changes with the planner before merging dependent work.

## Handoff

Use the [packet handoff format](docs/implementation/README.md#planner-handoff-template). State changed operations/files, evidence, results and unresolved issues. Do not open unrelated tickets, publish releases, or change infrastructure without authorization.
