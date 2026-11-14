# Kaneo comparison after independent design

Read-only comparison, 2026-11-14. The [independent baseline](independent-design.md) was written before opening Kaneo. No Kaneo files were changed. This is a design comparison, not a code audit or claim to have run its checks.

## Adopt

- `docs/architecture.md:5–25`: thin command boundary, private packages, exact JSON-number preservation, bounded bodies, isolated unauthenticated storage transport, no blanket retries.
- `docs/implementation.md:5–31`: bounded work packets, explicit acceptance, versioned evidence, no operation marked covered merely because it has a command registration.
- `docs/authentication.md:19–42`: atomic configuration writes, URL-bound credentials, safe file permissions/ownership, deliberate keyring-unavailable handling, no secrets in ordinary config.
- `.golangci.yml:1–31`: explicit high-signal linter allowlist and checks active in tests. Prefer this stable allowlist over evolving defaults or `enable-all`.

## Adapt

- Kaneo uses `profile` and a full `/api` base. Plane uses named `connection` records with an **instance base prefix**, then appends `/api/v1/` or `/api/v2/`; OAuth uses `/auth/o/` on that same configured prefix. The CLI must document and validate this distinction.
- Kaneo passes successful API JSON through directly; Plane wraps it in a stable CLI envelope to carry API version, pagination aggregation and accepted/partial metadata. Inner resource fields remain version-native.
- Kaneo has pinned OpenAPI + mapping/provenance files. Plane first needs a curated operation catalog from official docs because the public documented v2 schema returned 404. Keep snapshots reviewable and offline; no live-schema build dependency.
- Kaneo's keyring-first automatic plaintext fallback becomes explicit `env|keyring|file` selection for Plane. Silent downgrade would violate the desired configurable security boundary.
- Kaneo's detailed contract and verification history are large (the observed verification file was about 100 KB). Plane splits contract, packet evidence and source reference by task rather than growing one entry-point document indefinitely.

## Reject for this project

- `justfile:50–52` exposes hook setup, even though opt-in. Plane permits no hook machinery at all.
- Kaneo's documented device flow is not evidence Plane supports RFC 8628. Plane implements its own documented OAuth authorization-code/refresh and bot renewal flows.
- `docs/cli-contract.md:51` allows help/version to check GitHub for updates. Plane discovery must be offline, deterministic and credential-independent.
- Kaneo's six-target/Windows distribution is outside the selected four-target scope.
- Kaneo's live-instance acceptance gates do not apply: the user explicitly selected official-doc-backed fake-server acceptance for Plane.
- Do not copy Kaneo-specific endpoints, field names, warnings persistence, API-key scope semantics, or release migration prose into Plane contracts.

Source paths are local review provenance under `/home/blana/Projects/kaneo-cli`; the normative Plane docs contain the decisions so future implementers do not need that checkout.
