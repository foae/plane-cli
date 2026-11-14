# P01 — Connections, transport and basic credentials

Prerequisite P00. Read [authentication](../design/authentication.md), [CLI contract](../design/cli-contract.md), [architecture](../design/architecture.md), [verification](../verification.md).

## Own

`internal/config/`, `internal/httpx/`, `internal/auth/credentials.go`, CLI `connection.go` and basic auth status/import boundaries, output error/JSON integration, focused fixtures. Coordinate shared root/spec changes. Secure filesystem primitives belong in config/auth ownership, not another generic utility package.

## Ordered work

1. Implement config versioning, named connections and explicit defaults/precedence. Local repair commands resolve only their own settings. Validate/canonicalize base prefix and API generation; preserve unrelated entries under the configuration-file-wide read/modify/write lock and atomic replacement. Establish stable connection identity/generation and connection-before-config lock ordering for P02 lifecycle changes.
2. Implement env API-key/bearer and external auth modes. Separate selected auth mode from backend. External mode must bypass all local credential lookup even with token environment variables present. P02 adds persistence/OAuth; until then reject unavailable modes clearly rather than pretending they work.
3. Implement path/query construction, deadlines/context, proxy/custom CA behavior, stored credential binding, redirects and body limits. Accept configured HTTP without opt-in; warn only when establishing/changing an HTTP connection as specified in authentication. Environment tokens follow the selected invocation destination without an extra declaration. Reuse transports, close bodies, never retry automatically.
4. Translate v1/v2 failures without leaking raw bodies/secrets. Preserve native fields and JSON number precision. Implement staged JSON output, SIGPIPE/write failure handling and error precedence; input presence/duplicate-key/schema validation and destructive/dry-run/version guards precede credential access. Build the intentional-secret versus diagnostic redaction policy; actual secret-returning workflows arrive in their owning packets.
5. Build reusable two-origin fake-server and subprocess harnesses. Keep OAuth/storage clients separate policy boundaries; their actual workflows arrive later, not placeholder exported APIs now.

## Acceptance

A temporary fake-server endpoint exercised through a real test command harness proves prefix/escaping, API-key versus bearer exclusivity, external proxy injection, no stored credential forwarding to changed base, custom CA, redirect refusal and deadline/interrupt cleanup. HTTP works without flags or recurring warnings, including environment-only configuration. Two processes updating different connections preserve both changes. Use fresh subprocess environments and a non-loopback logical API hostname for proxy tests; map it through the fake proxy so Go's loopback bypass cannot invalidate the scenario. Test malformed/HTML/oversized errors with canary secrets, closed stdout and cleanup. No request/keyring access on bad input, dry-run or missing --yes. Delete throwaway probes; retain meaningful regressions.

Planner reviews transport/security boundaries before P02/P03. Do not add production `api request` as a shortcut for testing or coverage.
