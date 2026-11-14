# Connections, authentication and security

Approved design. Auth mode describes what is sent; storage describes where locally managed credentials live. They are independent.

## Named connections and precedence

`connection add NAME`, `update NAME`, `list`, `show [NAME]`, `use NAME`, `delete NAME --yes`. `add` requires `--base-url`, `--api-version v1|v2`; optional workspace/project/auth/backend defaults. No implicit Cloud destination. Missing explicit/default connection is actionable configuration error, unless all required invocation settings were supplied without a connection.

Settings resolve: explicit flag → nonempty `PLANE_CLI_*` env → selected connection → documented safe default. Connection selection: explicit `--connection` → `PLANE_CLI_CONNECTION` → saved default. Empty environment means unset. Effective `show` includes each nonsecret value and its source, never token/keyring lookup. Local list/delete/use and logout must remain usable when unrelated URL/timeout env overrides are malformed.

Nonsecret config: `os.UserConfigDir()/foae-plane-cli/config.json`; allow explicit `PLANE_CLI_CONFIG_DIR` for isolated automation/tests. Secrets are separate. No reading repository-local config or automatic Compose migration. Profile names are keys, never path components. Config schema has a version; unknown future versions fail without overwriting. File writes are locked/atomic and preserve unrelated connections.

Environment names: `PLANE_CLI_BASE_URL`, `API_VERSION`, `WORKSPACE`, `PROJECT`, `AUTH_MODE`, `CREDENTIAL_BACKEND`, `TIMEOUT`, `REQUEST_TIMEOUT`, `CA_FILE`, each with prefix `PLANE_CLI_`; secrets use `PLANE_CLI_API_KEY`, `PLANE_CLI_ACCESS_TOKEN`, `PLANE_CLI_OAUTH_CLIENT_SECRET`, `PLANE_CLI_OAUTH_REFRESH_TOKEN`. OAuth nonsecret client ID, redirect URI, scopes and installation ID can be stored in the connection or supplied via corresponding prefixed env. Never read Compose's `PLANE_*` credentials implicitly.

Base URL is an instance origin plus optional reverse-proxy prefix, e.g. `https://plane.example.net/plane`, **not** a dashboard path or `/api/v1` URL. Append `/api/{version}/...` or `/auth/o/...` while preserving the prefix. Reject query, fragment, userinfo, malformed/ambiguous path and already-versioned API suffixes with guidance. Normalize scheme/host case, effective port and trailing slash, not opaque path content.

## Auth modes

- `api-key`: `X-Api-Key` only. Obtain from selected secret source; missing credentials fail before request with setup guidance.
- `bearer`: `Authorization: Bearer` only, externally supplied access token; no assumed refresh protocol.
- `oauth-user`: registered app authorization-code login and refresh.
- `oauth-bot`: registered app client-credentials acquisition/renewal bound to an app installation.
- `external`: send neither authentication header, do not load a keyring, validate a local token, refresh OAuth or reject missing local auth. Ignore locally available token env values in this mode. Let the configured operator proxy inject credentials; upstream errors remain normal API errors. `auth status` says `externally_managed`, never “authenticated”.

For a local mode, its matching nonempty token environment value overrides the selected stored credential for this invocation only. Reject ambiguous simultaneous token overrides rather than guessing a different auth mode. Environment overrides are never persisted automatically. Changing effective base URL invalidates stored binding; require explicit rebind/login rather than forwarding an old token. Credential identity includes connection name, normalized base prefix, auth principal/mode and OAuth client/installation where applicable. API generation alone may share the same credential on the same base; scope/entitlement still belong to the server.

External mode is **not** TLS disablement. Honor standard `HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY` via `http.ProxyFromEnvironment`; support custom CA via `--ca-file`, appended to the system trust roots. Never log proxy credentials. An explicit plain-HTTP base is allowed for operator-controlled self-hosting; warn on each invocation that sends local credentials. Never downgrade HTTPS automatically. Out-of-process credential injection and proxy destination policy remain the operator's responsibility, including any credentials the proxy adds to unrelated traffic.

## Storage and lifecycle

`env` is default and writes no tokens. Explicit `keyring` stores under service `foae-plane-cli` with stable opaque binding keys; explicit `file` stores owner-only credentials under the config directory. No silent fallback between backends. A locked/denied/corrupt keyring is an error, not “not logged in”; advise explicit backend selection without leaking details. `auth status` shows mode/backend/binding/expiry only; `auth logout --yes` deletes this connection's local credentials and pending login state, not remote sessions.

Use 0700 directories, 0600 secret files and locks, safe ownership checks and refusal of symlink/unsafe parent paths for secret writes. Do not follow attacker-swapped links; use dirfd/no-follow primitives where needed. Atomic replacement and locking cover concurrent login/refresh/logout. File storage is plaintext at rest: say so explicitly before saving, using stderr diagnostics. Keyring selection does not prove a native daemon is available.

`auth login --api-key-file PATH|- --credential-backend keyring|file` imports a PAT without placing it on the process command line. OAuth persisted login likewise requires an explicit persistent backend; for `env`, token acquisition may write only to an explicitly supplied `--secret-output-file` with the same secure file rules, never stdout. Existing token env does not authorize persisting it during unrelated commands. Secrets may be intentionally delivered only through explicit protected output, including one-time upstream webhook/credential material; redact normal output and diagnostics.

Refresh uses a per-binding cross-process lock, rereads storage under lock, renews shortly before known expiry (30-second skew), stores rotated refresh tokens atomically and retains the prior refresh token when the server legitimately omits a replacement. Never refresh infinitely or replay an API request after 401. Expired external bearer tokens produce guidance to their owner. Failed renewal sends no API mutation. Invalid/revoked refresh credentials require explicit re-login.

## OAuth user login

Official [registration](https://developers.plane.so/dev-tools/build-plane-app/create-oauth-application) and [flows](https://developers.plane.so/dev-tools/build-plane-app/choose-token-flow) require operator app credentials. No shared embedded client secret, hosted broker, undocumented device grant or assumed dynamic registration. Do not treat the MCP server's OAuth proxy as the REST app provider.

`auth login --flow user` uses configured client ID, protected client secret, scopes and exact registered redirect URI:

1. Generate at least 32 random bytes of state; bind to connection/base/client/redirect, one-use login ID and a 10-minute local expiry. Never expose state or authorization codes in debug logs.
2. Build `/auth/o/authorize-app/` with `response_type=code`, client ID, exact redirect, space-separated scopes and state. Print deliberate consent instructions/URL on stderr; do not auto-open a browser. JSON final output is only safe login metadata.
3. Local mode listens only on the registered loopback host/port/path, validates GET callback, exact state, expiry and single use; rejects unexpected callbacks without consuming the valid pending login. Shut down promptly on completion/cancellation. A fixed registered URI is used, not an unregistered random port.
4. Exchange once at `/auth/o/token/`, form-encoded authorization_code, code, client_id, client_secret, redirect_uri as documented. Use an isolated HTTP client with redirects disabled, deadlines and reflected-secret suppression. Validate token_type/expiry and persist only after successful response validation.
5. Return safe identity of the connection/backend and expiry; no raw tokens. No requirement to preflight identity after login and no claim that consent guarantees every scope/feature.

Headless mode is explicit `auth login --flow user --headless`: securely persist pending state, emit the consent URL and login ID, then exit with `status:pending`. `auth complete --login-id ID --callback-file PATH|-` consumes the exact full redirected URI, validates its registered destination/state/code and exchanges once. Use an operator-owned registered redirect where the operator can obtain that callback; do not require a CLI-hosted public service. Delete pending state on completion/expiry/logout. Callback input is secret and cannot be a flag value. Manual completion does not bypass authorization, fabricate a code or prove that the deployment permits a chosen redirect URI.

PKCE/public-client and loopback registration support are not established by the inspected Plane docs. P02 must inspect further official evidence before enabling a public-client variant. Confidential bring-your-own-app mode is the baseline; report unsupported provider behavior plainly. Do not claim mock acceptance proves that a production registration UI accepts localhost.

## Bot flow

`auth login --flow bot` supports consent installation or a supplied app installation ID. Consent uses the same state protection/pending mechanics; callback supplies `app_installation_id`. Acquire at `/auth/o/token/` with HTTP Basic client_id/client_secret, form `grant_type=client_credentials`, `app_installation_id`, scopes. Parse the documented `access_token` field (despite “bot token” terminology). Renewal repeats the credential grant; it is not a user refresh-token request. Optional explicit installation inspection uses `/auth/o/app-installation/?id=...` and bearer auth; no hidden selection of a workspace from several installations.

## Transport isolation

API, OAuth and object-storage clients have separate policies. Reject redirects on ordinary API/OAuth requests; construct trailing-slash routes correctly. A documented download redirect is intercepted by the transfer workflow and followed via a separate unauthenticated client. Storage receives only the signed material required by that operation, never API auth or cookies; reject userinfo, disallowed scheme and unexpected redirects. TLS verification applies to all clients. Configured HTTP storage on a trusted self-hosted system requires explicit per-connection allowance, not a global insecure switch.

Always redact auth headers, token fields, proxy credentials, signed query parameters/form fields and callback values from stdout/stderr/debug/errors. Server-provided content is untrusted data. Never execute it, use it as a shell command or let it override connection/auth settings.
