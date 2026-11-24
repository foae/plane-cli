# Go architecture

Approved design. Read [CLI contract](cli-contract.md) for behavior and [API design](api.md) for wire differences. Do not create empty package scaffolding before a packet needs it.

## Ownership and dependency direction

```text
cmd/plane-cli → cli → config, auth, plane/v1, plane/v2, output, spec
plane/v1, plane/v2 → httpx
httpx → standard library (receives resolved auth; never reads config)
auth → config storage primitives and its own isolated OAuth client
output → standard library; spec → standard library, JSON Schema validator (neither uses HTTP/auth)
```

- `cmd/plane-cli/main.go`: signal context, real process IO, build metadata, single exit boundary. No resource logic; only here may call `os.Exit`.
- `internal/cli/root.go`, `run.go`: construct a fresh Cobra tree per invocation; `ExecuteContext`, `RunE`, `SilenceErrors`/`SilenceUsage`; translate one returned error into stderr/exit code. No global flags/state or `init` registration.
- `internal/cli/connection.go`, `auth.go`, `schema.go`: local commands; keep config repair/discovery independent from API validation or keyring access.
- `internal/cli/<domain>.go`: named command constructors and explicit field mapping. Split a large domain by actual responsibility, not one package per verb.
- `internal/config/{config,resolve,store}.go`: versioned JSON nonsecret config, precedence, canonical base URL and secure atomic persistence/locking.
- `internal/auth/{credentials,oauth,store,keyring,file}.go`: auth mode and backend resolution, token lifecycle, state, renewal and safe status. Platform-specific code uses build-tagged files only where required.
- `internal/httpx/{client,url,errors,transfer}.go`: contexts/timeouts, URL construction, auth injection, bounded decoding, redaction and redirect policy. No resource routes or Cobra dependencies.
- `internal/plane/v1/<domain>.go`, `internal/plane/v2/<domain>.go`: exact endpoint/request mapping; their own errors/pagination. Shared scalar helpers may live in `internal/plane` only when genuinely identical; do not normalize away version differences.
- `internal/output/{json,table,error}.go`: stable CLI envelope, redaction before serialization, table allowlisted projections, writer failure propagation. No network or stdout globals.
- `internal/spec/{catalog,schema}.go`: compiled offline operation metadata; one reviewed source drives discovery and coverage checks. Descriptors do not dynamically dispatch arbitrary URLs.
- `internal/testutil/`: reusable fake server/process helpers only, never production code. Domain fixtures remain beside their tests.
- `internal/cmd/checkdocs`, `internal/cmd/checkapi`: small development-only checkers when needed. Start with straightforward Make recipes; do not invent a task runner framework.
- `api/catalog.json`, `api/provenance.json`, `api/sources/`: future reviewed operation metadata and sanitized source snapshots. The current TSV is a discovery index, not this executable catalog.

### Offline metadata and validation

Use one reviewed catalog, not independently maintained schemas and flag tables. An explicit development-only generator (`internal/cmd/genspec`) deterministically writes `internal/spec/catalog.gen.json`, which `internal/spec` embeds with `go:embed`. Commit the generated file: tagged source and `go install` need neither generation nor repository-root reads. `checkapi` regenerates in a temporary directory and compares bytes; ordinary build/startup never writes or fetches metadata. Compile only implemented command descriptors into callable discovery; the source ledger also tracks pending work.

Descriptors declare command path tokens, per-version/scope schemas, flags (name/type/requiredness/body binding), sensitivity and availability. A shared Cobra registration helper consumes those flag descriptors; explicit Go handlers own endpoint calls and typed mappings. No reflective HTTP dispatcher. Version-exclusive flags can be registered in the command union, then validated against the selected descriptor before credentials. The checker rejects duplicate command/version/scope ownership, flags without mappings and handlers without descriptors.

An operation may declare `empty_success_status` only for a documented 2xx status with no response body. Transport accepts a zero-byte response at that exact status and emits JSON null; all other non-204 successes still require valid JSON and its media type. Do not enable empty responses globally or infer them from the HTTP method.

Validate parsed flag/body input against the same embedded JSON Schema 2020-12 returned by `schema`, using `github.com/santhosh-tekuri/jsonschema/v6`. Supply all references from embedded resources; install a loader that rejects external retrieval, and compile schemas lazily for the selected operation. Run duplicate-key/trailing-value checks before schema validation and retain exact `json.Number`/raw JSON semantics. Preserve documented extension fields. Do not turn library validation errors containing instance values into unsanitized diagnostics. P00 proves null/presence, references, composition and large-number boundaries before domain replication.

P00's flag-binding consistency checker follows direct `properties` chains with scalar types; required flags require every ancestor in that chain. Runtime validation supports embedded references/composition, but binding through composed/referenced properties is not yet supported. Resource packets must extend this checker with the first concrete schema requiring it, rather than bypass consistency checks. Non-local descriptors must declare a required string `api-version` selector bound to `/flags/api-version`.

## Go conventions

Follow [Code Review Comments](https://go.dev/wiki/CodeReviewComments), current standard-library conventions and [module layout guidance](https://go.dev/doc/modules/layout). `gofmt` owns formatting. Names are concise, packages lowercase/singular, exported declarations documented where non-obvious. Use `errors.Is`/`errors.As`, wrap with `%w` at useful boundaries, never string-match errors or panic for normal failures.

Pass `context.Context` first through IO calls; never store a request context in a long-lived struct. Close response bodies and files on every path. Check response status before normal payload decoding. Avoid unbounded goroutines, read-all on transfers, avoidable byte/string copies and repeated JSON conversions. Keep synchronous command execution unless measured/required concurrency exists.

Inject `*http.Client`, IO streams, clock/random seams where relevant and a small credential-store interface. Let consumers define interfaces; no interface for every struct, service locator, reflection dispatch, dependency-injection container or repository layer. No public SDK until a separately approved consumer requires one.

Use explicit small request structs for flags with presence represented separately from zero values. Use `json.RawMessage` for raw bodies/responses and `json.Number` when inspecting arbitrary numeric fields. A pointer alone does not distinguish omitted from explicit JSON null: retain presence bits/raw fields for PATCH. Do not round identifiers/counts through `float64`.

## Dependency policy

Initial pins are in [development](../development.md). Cobra for parsing/help; `x/oauth2` only for compatible standard exchange mechanics; `go-keyring` for platform backends; `x/sys/unix` for secure file/locking primitives where stdlib is insufficient. All direct and transitive dependencies remain recorded in go.mod/go.sum.

No Viper: stdlib JSON configuration plus explicit precedence is sufficient. No HTTP retry framework or external SDK. No YAML input requirement. No code generator until a concrete repetitive mapping benefits; generate offline reviewable metadata, not opaque runtime behavior. Review keyring implementations for process argument/secret exposure before adoption; protocol tests and cross-builds do not certify native keychain behavior.

## Concurrency and persistence

Use one advisory **configuration-file-wide** lock for every read/modify/write of `config.json`, including changes to different connections. Reread under lock, preserve unrelated entries, then atomically replace. A rename alone does not prevent lost updates. Use secure temporary siblings, fsync and directory sync where supported; stable lock files are not the replaced data files.

On first use, create the lock file exclusively. If another process creates it first, open that same file without `O_CREAT` and verify its ownership and permissions before locking. Concurrent nonexclusive creation can return `ENOENT` on macOS.

Credential bundles are separate per-binding records, never a single shared JSON secret map protected by unrelated locks. Serialize their login/refresh/logout/delete/rebind operations with a stable connection-name lock, independent of its current base/backend. Acquire with context/deadline. If both locks are needed, order is **connection lock → config-file lock**; never wait for a connection lock while holding the config lock, and never hold the config lock over network IO. Config-only reads/use may take only the config lock. Different connections may renew independently.

Under the connection lock reread configuration and credentials, verify the captured identity/generation and then decide whether renewal is still needed. Hold that lock through renewal and credential commit, not through the human consent wait. Every login completion rechecks its pending ID/generation; logout/delete/rebind invalidate pending attempts before releasing the lock. Removed/recreated connection names receive a fresh identity so an old login cannot resurrect them.

Configuration and secrets are separate stores; do not promise cross-store transactions. Save a newly created secret first, commit its reference second; on reference failure remove only the newly created secret and report cleanup failure. Backend migration retains the old reference until the new one is committed, then removes old credentials; a cleanup failure names the remaining backend without exposing values. Logout/deletion cleanup semantics are in [authentication](authentication.md#storage-and-lifecycle), including pending/staged state and failed cleanup. No remote revocation unless a separate documented command requests it.

## Testing boundaries

CLI process harness exercises real parsing, resolution, client, renderer and exit translation against `httptest.Server`; inject only the remote service and credential backend/clock. Transport tests assert security across two origins and cancellation. Domain fixtures assert exact method/path/query/body and observable data, not internal helper call counts. See [verification](../verification.md).
