# Acceptance and final review

Approved strategy: official documentation plus deterministic mocks/fake HTTP servers. **Live Plane end-to-end tests are not required.** Passing mocked protocols is not proof a particular Community/Commercial/Cloud deployment exposes them.

## Test at the consumer boundary

Use `httptest.Server` (and TLS servers where appropriate) with a real CLI parser, configuration resolution, HTTP client and renderer. Inject only remote endpoints, credential backend, deterministic clock/randomness and OS IO seams where necessary. Each request fixture asserts method, trailing-slash path, encoded query, selected auth header and exact meaningful body semantics; each scenario asserts stdout JSON, stderr/error code, files and exit status.

Use isolated temp directories, `t.Setenv` where safe, per-test servers and a fresh command tree. Do not combine process-global env mutation with parallel tests. No real home config/keyring, production network, wall-clock sleeps, global ports or credentials. Use fake clocks and deterministic cancellation synchronization. Binary process tests build once per package/test harness and prove actual exit/stream behavior; do not rebuild per table row.

Tests must catch plausible consumer failures, not source wording, registration counts, private helper shapes or tautological mocked return values. Group cases around behavior; fixture reuse must not hide per-operation method/body differences. Tests accompanying implementation are explicitly requested mock acceptance in this plan.

## Foundation acceptance matrix

| Area | Required scenarios | Evidence |
| --- | --- | --- |
| Offline discovery | Empty home, nested schema paths, version availability, installed binary outside repo | No requests/writes or root-file dependency |
| JSON/output | Large integer, unknown field, null vs 204, closed pipe/SIGPIPE | Exact result/status and cleanup |
| Input | PATCH omission/null/false/zero/empty, duplicate keys, invalid/trailing JSON | Correct bytes or no request |
| Destination | Prefix preservation, escaped IDs, malformed base, changed credential binding | Exact route/auth isolation |
| External auth | Fresh subprocess env, non-loopback logical host, ignored token env, proxy injection | API receives proxy auth only |
| TLS/proxy | Custom CA, untrusted TLS, NO_PROXY, configured HTTP without opt-in | Safe transport; setup-only HTTP warning |
| Secrets | Intentional JSON, ordinary-read redaction, protected-file preflight/delivery failure | No unintended disclosure or repeated mutation |
| Redirects | API/OAuth cross-origin refusal, storage isolation, same-origin policy | No leaked headers |
| Storage | Permissions/symlinks, concurrent updates to different connections, interrupted writes | No lost updates or corrupt files |
| Backends | File default, opt-in keyring, env overrides, external bypass | No silent downgrade or env-token refresh |
| OAuth user | Listener bind, state expiry/replay, headless cleanup, refresh without original env | Persisted bundle and safe lifecycle |
| OAuth bot | Installation grant and renewal, wrong installation, failure | Correct grant/header/body |
| Auth races | Refresh/logout/delete/recreated name, rotation persistence failure, lock timeout | No stale resurrection; partial outcome visible |
| Pagination | Both v2 styles, actual v1 shapes, empty page with next, repeated token | Correct bounded traversal |
| Failures | 401/402/403/404/409/429/5xx, HTML, malformed/oversized JSON | Stable safe error/exit |
| Cancellation | Deadline/interrupt during HTTP, login, wait/lock, transfer | Prompt stop and cleanup |
| Mutations | Missing --yes, dry-run, unknown outcome, documented partial/accepted cases | No unsafe request/replay; correct error precedence |
| Files | Three-phase upload, failed confirm, redirects, HTTP/private storage, overwrite | Exact bytes; no API credentials at storage |
| Tables | Declared columns, absent sparse fields, control characters | Stable safe projection |

Native keyring backends are covered through controlled doubles and cross-builds in required acceptance. Document that these do not prove an unlocked macOS Keychain or Linux Secret Service works on every machine. Optional manual native smoke or live Plane acceptance may add evidence later; neither is silently made a release gate against the user's instruction.

## Per-operation completion

A verified operation has reviewed source provenance, complete request/response/side-effect schema, a real leaf implementation, applicable table/help behavior, exact fake-server request fixture, meaningful success/error scenario and catalog entry linked to those scenarios. Shared harnesses are allowed; blindly generating tests from the same mapping under test proves little. Compare request expectations to independently reviewed official fixtures/contract excerpts.

Catalog promotion to `verified` requires links to the exact reviewed source/hash, fixture/scenario identifiers, owning packet and recorded acceptance result. A source-page alias is not a second operation. Accepted/partial/bulk scenarios are required only where that operation's evidence supports them; shared transport tests exercise generic mechanics without inventing upstream contracts. Review source licensing before committing excerpts or snapshots, retain required notices and record provenance.

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
- Release-version tooling enforces per-work-item SemVer impact, tag/metadata agreement and immutable `v1.0.0` promotion gates. The matching versioned agent skill passes packaged recipe/disclosure checks; explicit update checking passes fake-GitHub scenarios and never adds network access to ordinary commands. See [P10 slices](implementation/10-delivery.md#bounded-delivery-slices).
- No hooks, credentials in artifacts/fixtures, runtime schema download, automatic version fallback, hidden prompts or network-dependent help.
- Documentation examples/catalog agree with actual commands; root status changes from planned to implemented only when true. Compatibility notes explicitly say documentation-backed/mock-tested, not universally live-certified.

If official sources conflict or an operation cannot be mapped from accessible evidence, the implementation model reports exact evidence and asks the planner. It does not invent a fixture as proof of the guessed contract, waive coverage or downgrade scope without approval.
