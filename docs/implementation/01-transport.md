# P01 — Connections, transport and basic credentials

Prerequisite P00. Read [authentication](../design/authentication.md), [CLI contract](../design/cli-contract.md), [architecture](../design/architecture.md), [verification](../verification.md).

## Own

`internal/config/`, `internal/httpx/`, `internal/auth/credentials.go`, CLI `connection.go` and basic auth status/import boundaries, output error/JSON integration, focused fixtures. Coordinate shared root/spec changes. Secure filesystem primitives belong in config/auth ownership, not another generic utility package.

## Ordered work

1. Implement config versioning, named connections and explicit defaults/precedence. Local repair commands resolve only their own settings. Validate/canonicalize base prefix and API generation; preserve unrelated entries under locked atomic writes.
2. Implement env API-key/bearer and external auth modes. Separate selected auth mode from backend. External mode must bypass all local credential lookup even with token environment variables present. P02 adds persistence/OAuth; until then reject unavailable modes clearly rather than pretending they work.
3. Implement path/query construction, deadlines/context, proxy/custom CA behavior, credential binding, redirects and body limits. Use reusable HTTP transports, always close bodies and never retry requests automatically.
4. Translate v1/v2 failures without leaking raw bodies/secrets. Preserve response bytes/native fields and JSON number precision. Implement staged JSON output and writer failure handling; implement input presence/duplicate-key checks and destructive/dry-run guards before credential access.
5. Build reusable two-origin fake-server and subprocess harnesses. Keep OAuth/storage clients separate policy boundaries; their actual workflows arrive later, not placeholder exported APIs now.

## Acceptance

A temporary fake-server endpoint exercised through a real test command/harness proves prefix/escaping, API-key versus bearer exclusivity, external proxy injection, no stored credential forwarding to changed base, custom CA, redirect refusal and deadline/interrupt cleanup. Test malformed/HTML/oversized errors containing canary secrets. Demonstrate no request/keyring access on input error, dry-run or missing --yes. Use the matrix's config/permission/concurrency cases. Delete throwaway probes; retain meaningful regression fixtures.

Planner reviews transport/security boundaries before P02/P03. Do not add production `api request` as a shortcut for testing or coverage.
