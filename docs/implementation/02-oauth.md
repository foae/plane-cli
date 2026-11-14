# P02 — Persistent credentials and full OAuth

Prerequisite P01. Read [authentication](../design/authentication.md) in full, [verification](../verification.md), and exact official registration/token-flow pages linked there. Do not borrow Kaneo device-login assumptions.

## Own

`internal/auth/{store,keyring,file,oauth}.go` and narrowly needed platform files; CLI `auth.go`; auth/config fixtures; direct auth dependencies and docs through the coordinator. Use isolated secret stores, never a developer's real keychain.

## Ordered work

1. Verify source evidence for user authorization, refresh, bot consent/client_credentials, registration/callback requirements, scopes and token revocation if documented. Record exact forms, headers, response fields and expiry semantics. Public-client PKCE/device support is an evidence question, not an implemented assumption.
2. Implement selectable env/keyring/file backends, protected PAT import/output, secret binding and explicit migrations. Test 0700/0600, unsafe/symlink paths, atomic writes, unavailable keyring and nonsecret status. Backend failures must not downgrade to plaintext.
3. Implement user consent and exact registered loopback callback; cryptographic state, expiry, replay protection and one exchange. Implement headless pending-state/complete lifecycle without putting callback/code/secret on argv. Consent URL exposure is deliberate; arbitrary logging is not.
4. Implement bot consent or supplied installation ID, correct Basic client credential grant, renewal and optional explicit installation read. Never label bot renewal a refresh-token flow.
5. Serialize refresh/logout/login under per-binding cross-process locks; reread before renewal, preserve/rotate refresh tokens correctly, and prevent stale writes from resurrecting logged-out credentials. Never replay an API mutation after auth failure.
6. Validate redirect/destination, token response and safe error handling; bound waits and clean pending state on completion/cancel/expiry. Environment-only acquisition uses explicit protected output, never raw tokens on stdout.

## Acceptance

Fake OAuth providers cover successful/denied/expired/mismatched/replayed user callbacks; manual completion; user refresh rotation/omission/invalid grant; bot install/renewal; cross-origin redirects; two-process refresh and logout races; interrupted login and lock timeout. All paths assert no secrets on normal streams. Mock credential backend plus Unix permission tests prove storage policy, not native Keychain/Secret Service availability.

Planner security review is mandatory before resource expansion. If exact registration semantics remain deployment-dependent, document that limitation and the configured confidential-client baseline; do not require live E2E acceptance or assert universal provider support.
