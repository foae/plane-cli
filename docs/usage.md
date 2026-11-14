# CLI usage

P00/P01 implement offline discovery, named connections, environment credentials and the safe API transport foundation. No resource operations are callable yet; persistent login/OAuth and resource commands belong to P02/P03.

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

`commands [GROUP]` lists only implemented command summaries and explicit version availability. `schema <command-path...>` resolves the entire canonical path; its `.data.descriptor.schema` is the same JSON Schema used to validate the command's inputs. The input object has `arguments` (positional tokens) and `flags` (explicit values, plus the effective API version for resource commands). Discovery requires explicit `--api-version v1|v2`.

Unknown command paths (including help topics), invalid UTF-8 arguments or unknown flags return exit 2 and one safe JSON error on stderr, with empty stdout. Missing required flags are named without their values. A known descriptor unavailable in the selected version returns exit 7. Output failure or invalid embedded schemas return exit 1; interrupt cancellation returns 130 unless the command has already completed successfully. Known remote mutation followed by delivery failure returns exit 6.

Schemas resolve only embedded resources. JSON parsing rejects duplicate keys and trailing values while preserving exact numbers and the distinction between omitted properties, null and explicit zero values. No schema is fetched at runtime.

## Connections and environment credentials

```sh
plane-cli connection add local --base-url http://plane.example.net/plane --api-version v2 --auth-mode external
plane-cli connection use local
plane-cli connection show
plane-cli connection list
plane-cli auth status
plane-cli connection delete local --yes
```

Configuration lives in the OS user config directory under `foae-plane-cli`, or `PLANE_CLI_CONFIG_DIR`. Directories must be owner-only (0700); files and stable locks are 0600. Symlink/unsafe parents are refused; use a canonical path when a system directory is an alias (such as `/private/tmp` on macOS). Concurrent updates preserve unrelated connections. Stable lock files remain after deletion so existing waiters and new callers cannot acquire different lock inodes. Profile names are opaque map keys, not filesystem paths. Unknown configuration fields are rejected rather than silently erased; records larger than 8 MiB are refused before replacement.

Settings resolve explicit flag, nonempty `PLANE_CLI_*` environment, selected connection, then safe default. API generation and destination have no implicit default. `connection show` reports nonsecret values and their sources without reading tokens. List/use/delete ignore unrelated malformed environment overrides. HTTP works without opt-in; only adding an HTTP connection or changing HTTPS to HTTP emits a structured informational warning.

Resource dispatch resolves the API version from flags, environment or the selected connection before choosing its schema. A fully specified invocation without an explicit connection can run when local configuration is unavailable. A selected named connection never silently falls back. Dry-run validates route segments and reports the target URL and whether a body is present, without reading credentials.

`api-key` uses `PLANE_CLI_API_KEY`; `bearer` uses `PLANE_CLI_ACCESS_TOKEN`. Both together are rejected. `external` ignores both and sends neither header, allowing an operator proxy to inject authentication; status says `externally_managed`, not authenticated. Persistent file/keyring credentials and OAuth are not available in P01. Backend selection is stored but never silently falls back.

Transport honors standard proxy variables, verifies HTTPS certificates and appends `--ca-file` roots. API redirects are refused. Total/request defaults are 5m/30s; both require positive durations. There are no automatic retries: transport uses HTTP/1 only and disables keep-alive pooling to prevent Go's HTTP/2 stream retries and stale pooled-connection replays. JSON responses retain native fields and exact numbers, are limited to 16 MiB, and are staged before stdout. Server-error responses to mutations report an unknown outcome. Arbitrary upstream error text is suppressed rather than exposing reflected secrets.

The [packet router](implementation/README.md) identifies remaining work. Deterministic fake-server acceptance does not establish live Plane edition compatibility.
