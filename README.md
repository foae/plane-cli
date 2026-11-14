# Plane CLI

An agent-first Go CLI planned for Plane Cloud and self-hosted Community and Commercial editions.

**Status: design and implementation plan only.** No binary, Go implementation, Make recipes or release artifacts are implemented here yet.

The approved design provides explicit API v1/v2 connections, full documented public API coverage, JSON-first results, optional tables, offline command schemas, and secure headless authentication—including external credential injection. Distribution targets Linux/macOS amd64/arm64 through release archives, `go install`, and Homebrew.

- **Implement:** start at [AGENTS.md](AGENTS.md), then take one [work packet](docs/implementation/README.md).
- **Understand the plan:** [documentation router](docs/README.md).
- **Review decisions:** [scope and decisions](docs/planning/decisions.md).
- **Evaluate evidence:** [research and limitations](docs/reference/research.md).

Documentation uses progressive disclosure: start small, follow task-specific links, and load deep API references only when needed. There are no git hooks; the [development plan](docs/development.md) defines explicit feedback recipes.
