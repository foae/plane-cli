# Independent design baseline

Recorded 2026-11-14 **before inspecting `/home/blana/Projects/kaneo-cli`**. This is a planning artifact, not a claim that any commands exist. Subsequent decisions belong in the decision log; normative contracts will live in the linked design documents.

## Product decisions from the first three interviews

- Binary `plane-cli`; Go module `github.com/foae/plane-cli` (repository origin); retain MIT license.
- Full documented public REST API coverage, not just work items. Cloud, self-hosted Community, and self-hosted Commercial are target deployments. Their actual resource availability differs.
- Explicit API v1/v2 selection per connection. No silent fallback, payload translation, or replay against a second API version.
- Standalone CLI, no MCP server, TUI, hosted service, or declarative project synchronizer.
- JSON by default, no implicit terminal prompts; common field flags plus complete JSON file/stdin input. Normal create/update execute; destructive commands require `--yes`.
- Lists return one bounded page; explicit `--all` traverses. Native bulk operations are in scope; a generic workflow engine is not.
- Linux/macOS amd64/arm64 releases. No Windows requirement.
- PAT, bearer injection, and full bring-your-own-app OAuth login/renewal. Credentials selectable from environment, OS keyring, or owner-only files. Also explicit external-auth mode: send no local authentication header and let an operator-managed proxy inject credentials. Do not preflight-reject such requests for lack of a token.
- Acceptance uses official documentation and deterministic fake HTTP servers. The user has a self-hosted Commercial Free instance available later, but live end-to-end tests are not required and must not become a release gate in this plan.
- No git hooks. Explicit, tiered Make recipes and matching CI checks.
- Progressive disclosure is a project rule: short root guidance, task router, focused contracts, bounded implementation packets, deep source inventory only on demand.

## Evidence shaping the design

1. [Plane v2 introduction](https://developers.plane.so/api-reference/v2/introduction.md) calls v2 current, with v1 retained. [Migration](https://developers.plane.so/api-reference/v2/migrating-from-v1.md) documents different field names, sparse responses, pagination and v1-only operations. Versions need separate adapters, not an assumed common wire model.
2. [Editions](https://developers.plane.so/self-hosting/editions-and-versions.md) have separate codebases/release cycles. No established minimum Community version for v2 was found. Declare compatibility evidence honestly; do not infer a feature is unavailable from a tenant-hidden 404.
3. The documented `https://api.plane.so/api/v2/schema/` returned HTTP 404 with `{"type":"not_found","code":"not_found","detail":"The requested resource was not found."}` during unauthenticated research. This does not establish that v2 itself is absent. Do not make build, startup or local help depend on a live schema.
4. [OAuth flow documentation](https://developers.plane.so/dev-tools/build-plane-app/choose-token-flow) documents authorization-code/refresh and bot client-credentials renewal, both with operator app credentials. Public-client PKCE/device flow support is unverified. Never embed a shared client secret or pretend this is device-code authentication.
5. [Plane Compose](https://developers.plane.so/dev-tools/plane-compose) already owns `plane` and formerly used `~/.config/plane-cli`. Use the selected `plane-cli` binary and a distinct `foae-plane-cli` configuration namespace; never auto-import Compose credentials.
6. [Go module layout](https://go.dev/doc/modules/layout) supports `internal` packages without inventing a public SDK. [golangci-lint](https://golangci-lint.run/docs/) v2 supports explicit high-signal lint configuration; formatter mutation must be separate from CI checks.

## Independent architecture

Use Cobra for the command tree, standard-library HTTP/JSON/context, and small explicit packages. No Viper, generic repository/service layers, runtime OpenAPI command generation, or broad SDK dependency without demonstrated need.

```text
cmd/plane-cli/main.go       process exit, signals, version injection
internal/cli/              Cobra constructors, flags, help, dispatch
internal/config/           connection resolution, precedence, safe local config
internal/auth/             credential backends, OAuth state/exchange/refresh
internal/httpx/            authenticated API transport, limits, errors, redaction
internal/plane/v1/          version-specific requests and pagination
internal/plane/v2/          version-specific requests and pagination
internal/output/           stable envelope, structured errors, text projection
internal/spec/             offline operation descriptors and command schema
internal/testutil/         fake HTTP servers and isolated process harness
```

Share transport, not wire assumptions. Keep handlers thin but avoid one interface per struct. Interfaces belong at genuine IO/test seams. Preserve absent/null/false/empty-list distinctions in PATCH. Preserve unknown response fields and JSON number precision; resource data stays version-native inside a stable CLI envelope.

Offline discovery: root help lists groups, group help lists commands, leaf help gives required inputs/examples, `schema <command...>` returns only that operation's machine contract. An explicit command catalog lists summaries without loading every schema into an agent's context. Unknown/unsupported operations fail visibly, never masquerade as empty success.

Security boundaries: base URLs are operator configuration, not response-controlled URLs. TLS verification remains enabled, with custom CA support. API credentials never follow cross-origin redirects or storage upload/download requests. Remote descriptions are untrusted data, not instructions. No shell execution from configuration or API content. Secrets never appear in debug output, dry-run output, fixtures, or ordinary schemas.

Reliability: context deadlines/cancellation, bounded bodies, native server pagination tokens, no hidden unbounded traversal, and no automatic mutation replay after uncertain transport outcomes. Explicit dry-run assembles and validates without sending writes; it is not a promise of server authorization or transactional execution.

## Independent implementation sequence

1. Establish the offline command/output contract, endpoint evidence inventory, module/tool pins, root CLI, Make recipes and CI. No hooks.
2. Implement connection selection, all credential backends including external mode, and a single safe HTTP transport.
3. Implement OAuth user login/refresh and bot acquisition/renewal, isolated storage/refresh locking, and headless setup contracts.
4. Implement separate v1/v2 pagination/errors plus a complete vertical slice: identity, projects, states, labels, work-item create/read/update/delete, with fake-server acceptance.
5. Expand core planning and work-item subresources, types/properties, pages/collections/assets, initiatives/releases/customers, and workspace/admin/automation/integration/agent surfaces. Each operation must have exact source evidence, schema/help, request mapping, and observable acceptance before it is considered covered.
6. Complete native bulk semantics, attachment multi-step workflows, coverage reconciliation, release artifacts and final senior review. These are part of the full release, not optional follow-ups.

A cheaper implementation model receives a packet naming prerequisite contracts, exact files, bounded operations, source references, accepted request/response/error cases, verification recipe and stop/escalation conditions. It must not decide unresolved product or security policy. The planner reviews each boundary-changing packet and the final assembled product.

## Questions retained for the comparison/final interview

- Whether an optional human-readable output mode is useful enough to retain (JSON remains the default).
- Which initial working slice the user prefers; this changes order, not full-release scope.
- OAuth redirect registration constraints vary by deployment; document supported callback/manual completion mechanics without claiming unverified server support.

The comparison with Kaneo must report what to adopt, adapt and reject against this baseline, not retroactively rewrite the independent rationale.
