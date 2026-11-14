# Acceptance and final review

Approved strategy: official documentation plus deterministic mocks/fake HTTP servers. **Live Plane end-to-end tests are not required.** Passing mocked protocols is not proof a particular Community/Commercial/Cloud deployment exposes them.

## Test at the consumer boundary

Use `httptest.Server` (and TLS servers where appropriate) with a real CLI parser, configuration resolution, HTTP client and renderer. Inject only remote endpoints, credential backend, deterministic clock/randomness and OS IO seams where necessary. Each request fixture asserts method, trailing-slash path, encoded query, selected auth header and exact meaningful body semantics; each scenario asserts stdout JSON, stderr/error code, files and exit status.

Use isolated temp directories, `t.Setenv` where safe, per-test servers and a fresh command tree. Do not combine process-global env mutation with parallel tests. No real home config/keyring, production network, wall-clock sleeps, global ports or credentials. Use fake clocks and deterministic cancellation synchronization. Binary process tests build once per package/test harness and prove actual exit/stream behavior; do not rebuild per table row.

Tests must catch plausible consumer failures, not source wording, registration counts, private helper shapes or tautological mocked return values. Group cases around behavior; fixture reuse must not hide per-operation method/body differences. Tests accompanying implementation are explicitly requested mock acceptance in this plan.

## Foundation acceptance matrix

| Area | Required scenarios | Evidence |
| --- | --- | --- |
| Offline discovery | Empty home, no credentials/network, group/leaf help and schema | No requests/writes |
| JSON/output | Large integer, unknown field, null vs 204, writer failure | Exact decoded result/status |
| Input | PATCH omission/null/false/zero/empty, duplicate keys, invalid/trailing JSON | Correct bytes or no request |
| Destination | Prefix preservation, escaped IDs, malformed base, changed credential binding | Exact route/auth isolation |
| External auth | No secret source, token env present but ignored, proxy injection | API receives expected proxy auth only |
| TLS/proxy | Trusted custom CA, untrusted TLS, NO_PROXY, credentialed proxy redaction | Safe transport/error |
| Secrets | Known token in upstream error, signed URL, secret-returning response | No unintended disclosure |
| Redirects | API/OAuth cross-origin refusal, storage isolation, same-origin policy | No leaked headers |
| Storage | 0700/0600, symlink/ownership refusal, interrupted/concurrent write | Valid old/new state only |
| Backends | Env/keyring/file/external selection, unavailable/denied keyring | No silent downgrade |
| OAuth user | Consent, state mismatch/expiry/replay, denied callback, refresh rotation | Safe state and persisted token |
| OAuth bot | Installation grant and renewal, wrong installation, failure | Correct grant/header/body |
| Refresh races | Two processes, rotated token, logout race, lock timeout | No stale token resurrection |
| Pagination | Both v2 styles, actual v1 shapes, empty page with next, repeated token | Correct bounded traversal |
| Failures | 401/402/403/404/409/429/5xx, HTML, malformed/oversized JSON | Stable safe error/exit |
| Cancellation | Deadline/interrupt during HTTP, login, wait/lock, transfer | Prompt stop and cleanup |
| Mutations | Missing --yes, dry-run, timeout after body sent, native partial bulk | No unsafe request/replay |
| Files | Three-phase upload, failed confirm, redirected download, overwrite | Exact bytes and partial status |
| Tables | Declared columns, absent sparse fields, control characters | Stable safe projection |

Native keyring backends are covered through controlled doubles and cross-builds in required acceptance. Document that these do not prove an unlocked macOS Keychain or Linux Secret Service works on every machine. Optional manual native smoke or live Plane acceptance may add evidence later; neither is silently made a release gate against the user's instruction.

## Per-operation completion

A verified operation has reviewed source provenance, complete request/response/side-effect schema, a real leaf implementation, applicable table/help behavior, exact fake-server request fixture, meaningful success/error scenario and catalog entry linked to those scenarios. Shared harnesses are allowed; blindly generating tests from the same mapping under test proves little. Compare request expectations to independently reviewed official fixtures/contract excerpts.

A family is complete only when all indexed/supplemental operations are accounted for, including non-CRUD actions and version-specific exceptions. Unsupported **deployment** capabilities are legitimate upstream errors; missing **CLI** mappings within approved scope are unfinished work. Never label an entire family complete because one CRUD path works.

## Planner review gates

1. P00: CLI envelope, exit/safety policy, command mapping conventions, provenance/catalog format and tool/CI baseline.
2. P01/P02: credential binding, proxy/external behavior, secure storage and OAuth mechanics before mass command work.
3. P03: one complete dual-version work-item lifecycle demonstrates architecture and mocks without avoidable abstraction.
4. P04–P09: bounded domain packets; verify exact evidence, typed input presence, pagination and side effects. Review shared contract changes before dependents.
5. P10: reconcile all scope, run release acceptance, review dependency/licensing/security/tool results and install artifacts. No publication without separate authorization.

## Final gate

- Every requested public operation is either verified against the documented mock contract or an explicitly unresolved evidence conflict presented to the user; unresolved required operations mean the full release is **not complete**.
- `make check`, race, security and four-target cross-build pass; native fake-server CLI tests pass on Linux/macOS. Record actual commands/versions/results; no rerun just to relabel a failure.
- Snapshot archives/checksums/version metadata, `go install` path and Homebrew formula are exercised per [releases](releases.md).
- No hooks, credentials in artifacts/fixtures, runtime schema download, automatic version fallback, hidden prompts or network-dependent help.
- Documentation examples/catalog agree with actual commands; root status changes from planned to implemented only when true. Compatibility notes explicitly say documentation-backed/mock-tested, not universally live-certified.

If official sources conflict or an operation cannot be mapped from accessible evidence, the implementation model reports exact evidence and asks the planner. It does not invent a fixture as proof of the guessed contract, waive coverage or downgrade scope without approval.
