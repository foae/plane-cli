# Plane CLI

An agent-first Go CLI for Plane, with an offline foundation available now.

**Status: P00 foundation.** The binary implements help, structured version output, command discovery and offline schemas. Plane resource commands, connections and authentication are not implemented.

Build with `make build`, then run `bin/plane-cli --help`. See [offline usage](docs/usage.md) and [contributor setup](docs/development.md). The approved design targets explicit API v1/v2 connections and documented public API coverage on Linux/macOS amd64/arm64; it is not a claim of current API or edition compatibility.

- **Implement:** start at [AGENTS.md](AGENTS.md), then take one [work packet](docs/implementation/README.md).
- **Understand the plan:** [documentation router](docs/README.md).
- **Review decisions:** [scope and decisions](docs/planning/decisions.md).
- **Evaluate evidence:** [research and limitations](docs/reference/research.md).

Documentation uses progressive disclosure: start small and follow task-specific links. There are no git hooks; [development recipes](docs/development.md) provide explicit feedback.
