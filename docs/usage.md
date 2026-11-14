# Offline CLI usage

P00 implements local discovery only. It does not connect to Plane, read credentials or create configuration. No resource operations are callable; the source ledger tracks their pending owners separately.

Build with Go 1.27.4 and `make build`, or install from this source checkout with `go install ./cmd/plane-cli`. Generated metadata is committed and embedded: the installed binary does not need repository files. Contributor setup and checks are in [development](development.md).

```sh
plane-cli --help
plane-cli version
plane-cli --version
plane-cli commands --api-version v2
plane-cli commands schema --api-version v1
plane-cli schema commands --api-version v2
plane-cli schema schema --api-version v1
```

No arguments, `--help` and `help [command-path...]` print human-readable help. Other successful commands emit one JSON envelope with `schema_version`, `data`, and local `meta.command`/`meta.status`. Local metadata never invents HTTP status or API response information. `version` and `--version` produce the same build metadata; source builds report `dev`/`unknown` until release metadata is supplied by the P10 packaging workflow.

`commands [GROUP]` lists only implemented command summaries and explicit version availability. `schema <command-path...>` resolves the entire canonical path; its `.data.descriptor.schema` is the same JSON Schema used to validate the command's inputs. The input object has `arguments` (positional tokens) and `flags` (only explicitly supplied flag values). Descriptor bindings identify body properties for future resource commands. Discovery requires explicit `--api-version v1|v2`; no connection settings exist yet.

Unknown command paths (including help topics), invalid UTF-8 arguments or unknown flags return exit 2 and one safe JSON error on stderr, with empty stdout. Missing required flags are named without their values; command suggestions contain only registered names. A known descriptor unavailable in the selected version returns exit 7 (the three implemented local commands currently support both versions). Output failure or invalid embedded schemas return exit 1; interrupt cancellation returns 130 unless the command has already completed successfully. Arbitrary argument values are not reflected in diagnostics.

Schemas resolve only embedded resources. JSON parsing rejects duplicate keys and trailing values while preserving exact numbers and the distinction between omitted properties, null and explicit zero values. No schema is fetched at runtime.

Pending API examples in design documents are not runnable commands. The [packet router](implementation/README.md) identifies the remaining implementation work. The P00 catalog/conventions/toolchain require planner approval before P01; the foundation does not establish live Plane edition compatibility.
