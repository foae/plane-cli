# Documentation router

The CLI provides offline discovery, named connections, authentication and documented P03–P09 resource operations. [Current usage](usage.md) describes working commands; [coverage exceptions](reference/api-contract-gaps.md) distinguish upstream evidence gaps from implemented API coverage.

## Choose your task

- Use the CLI → [usage](usage.md).
- Product scope, choices, exclusions → [decisions](planning/decisions.md).
- Implement a bounded slice → [packet sequence](implementation/README.md), then only its prerequisites.
- Command names, inputs, JSON, errors, tables → [CLI contract](design/cli-contract.md).
- Packages, Go conventions, dependency direction → [architecture](design/architecture.md).
- Connections, proxies, secrets, login and refresh → [authentication](design/authentication.md).
- API generations, pagination, coverage and difficult operations → [API design](design/api.md).
- Format, lint, test, CI, reproducible tool setup → [development](development.md).
- Mock acceptance and final review → [verification](verification.md).
- Archives, `go install`, Homebrew → [releases](releases.md).
- Agent skill layout, progressive disclosure and compatibility → [skill delivery plan](design/agent-skill.md).
- Why these decisions were made → [research](reference/research.md), [independent baseline](planning/independent-design.md), [Kaneo comparison](planning/kaneo-comparison.md).

## Disclosure rules for every contribution

Keep entry points short and task-oriented. A contract has one canonical home. A packet links prerequisites instead of copying them. Source evidence stays in reference material; historical reasoning stays in planning artifacts. User guides describe implemented behavior only, never advertise unfinished packets.

The [API source index](reference/api-index.tsv) is deep reference, not a document to read wholesale. It has 987 documentation links (649 v2, 338 v1), **not 987 verified operations**. Filter by version and group as shown in [API design](design/api.md#source-inventory-and-coverage).

When implementation starts, add focused getting-started and task-recipe pages using working examples; generate per-group command reference from reviewed offline descriptors. Do not create empty guides ahead of working behavior. Help and schemas must remain usable with network disabled and no credentials/configuration present.
