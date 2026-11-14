# P02 — Persistent credentials and full OAuth

Prerequisite P01. Read [authentication](../design/authentication.md) in full, [verification](../verification.md), and exact official registration/token-flow pages linked there. Do not borrow Kaneo device-login assumptions.

## Own

`internal/auth/{store,keyring,file,oauth}.go` and narrowly needed platform files; CLI `auth.go`; auth/config fixtures; direct auth dependencies and docs through the coordinator. Use isolated secret stores, never a developer's real keychain.

## Ordered work

1. Verify source evidence for user authorization, refresh, bot consent/client_credentials, registration/callback requirements, scopes and token revocation if documented. Record exact forms, headers, response fields and expiry semantics. Public-client PKCE/device support is an evidence question, not an implemented assumption.
2. Implement default owner-only file persistence, opt-in keyring, environment overrides, protected PAT/client-secret import, complete credential bundles and explicit migrations. Explicit env-backed login fails before HTTP with persistent-backend guidance; environment access-token overrides remain externally managed and never consume stored refresh tokens. Test permissions, symlinks, atomic writes and unavailable keyring without backend fallback.
3. Implement user consent; bind the registered loopback listener before exposing consent. Implement cryptographic state, expiry, single-use callbacks, staged app-secret storage and headless pending/complete lifecycle bound to connection generation. No callback/code/secret on argv. Consent URL exposure is deliberate; arbitrary logging is not.
4. Implement bot consent or supplied installation ID, correct Basic client credential grant, automatic renewal and optional explicit installation read. User login automatically refreshes from its persisted bundle, including client credentials, without requiring the original environment. Never label bot renewal a refresh-token flow.
5. Apply connection-scoped cross-process locks and connection-before-config ordering to refresh/logout/login/rebind/delete. Reread before renewal; persist rotation and prevent stale login/refresh writes after logout/deletion. Do not hold locks during consent waiting or replay API mutations after auth failure.
6. Validate token responses and safe errors; bound waits and clean expired/cancelled/consumed pending records plus staged secrets. Implement connection deletion and partial cleanup repair. Successful token rotation followed by failed persistence reports a partial outcome and fresh-login guidance, never retries the consumed refresh token.

## Acceptance

Fake OAuth providers cover successful/denied/expired/mismatched/replayed callbacks; bind failure before consent; headless completion; default file login and later refresh in a new process without the original app-secret environment; rotation/omission/invalid grant; bot renewal; cross-origin redirects; two-process refresh/logout/delete/recreated-name races; interrupted login and lock timeout. Environment overrides send only the supplied access token and cause no refresh/store writes. Failed persistence after successful rotation is partial. Auth streams contain no saved tokens; pending consent URLs are the documented exception. Mock backends plus Unix permission tests prove storage policy, not native Keychain/Secret Service availability.

Planner security review is mandatory before resource expansion. If exact registration semantics remain deployment-dependent, document that limitation and the configured confidential-client baseline; do not require live E2E acceptance or assert universal provider support.
