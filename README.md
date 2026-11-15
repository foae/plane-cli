# Plane CLI

An agent-first, JSON-first CLI for [Plane](https://plane.so), for humans, scripts and AI coding agents. Manage projects, work items, cycles, pages, customers, releases and more through API v1/v2 connections, defaulting to v2.

## Install

**Homebrew (Linux and macOS):**

```sh
brew install foae/tap/plane-cli
```

**Go (Go 1.27.4 or later):**

```sh
go install github.com/foae/plane-cli/cmd/plane-cli@v1.0.3
```

Or download a Linux/macOS **amd64 or arm64** archive from [Releases](https://github.com/foae/plane-cli/releases/latest). Verify it against `SHA256SUMS`, extract it, and put `plane-cli` on your `PATH`. The matching agent skill is a separate `plane-cli-skill_1.0.3.tar.gz` asset.

## First request

Replace the server URL and workspace slug below with your own. API selection resolves explicit `--api-version` → nonempty `PLANE_CLI_API_VERSION` → saved selected connection → `v2`, for execution and offline discovery. Saved v1 connections remain v1. New connections resolve `--api-version` → nonempty `PLANE_CLI_API_VERSION` → `v2`, without inheriting the selected connection. Updates retain the stored version unless explicitly changed. Invalid explicit versions are rejected; the CLI never automatically falls back between versions.

```sh
plane-cli connection add personal --base-url https://plane.example.net
plane-cli connection use personal
plane-cli auth login --connection personal --api-key-file -
# Supply your API key on stdin, then EOF. Do not put secrets in command arguments.
plane-cli user me
plane-cli project list --workspace YOUR_WORKSPACE
```

Credentials are stored in owner-only files by default. OS keyring storage is opt-in; environment injection and OAuth are also supported. See [authentication and connections](docs/usage.md#connections-and-environment-credentials).

## Discover before acting

Help and JSON schemas work offline, without credentials:

```sh
plane-cli --help
plane-cli commands
plane-cli work-item --help
plane-cli schema work-item create
plane-cli version
```

Commands return JSON by default. Use `--body-file` for native JSON request bodies, `--dry-run` to validate resource operations without sending them, and explicit `--yes` for destructive actions. [Usage and recipes](docs/usage.md) cover scopes, pagination, tables and exit codes.

## Coverage and safety

Documented operations are implemented across API v1/v2, with [27 approved upstream contract exceptions](docs/reference/api-contract-gaps.md). Unsupported operations are not guessed or silently redirected. Deterministic fake-server acceptance verifies protocols; it does **not** establish live Plane edition compatibility. Native keyring smoke testing remains separate.

Updates are explicit: `plane-cli update check` checks for releases but never installs anything. Upgrade using your original installation method.

## Agents and contributors

- **Agent skill:** download the matching release skill archive; follow its `SKILL.md` and task-specific references.
- **Build and contribute:** [development setup and checks](docs/development.md). Start with `make build`, then `bin/plane-cli --help`.
- **Architecture and contracts:** [documentation router](docs/README.md).
- **Implementation guidance:** [AGENTS.md](AGENTS.md) and [work packets](docs/implementation/README.md).
- **Release verification:** [distribution and provenance](docs/releases.md).

Licensed under [MIT](LICENSE).
