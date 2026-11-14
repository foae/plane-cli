# Implementation packet router

Plan only: no Go implementation has begun. Complete packets in dependency order; a packet may be divided into source-family slices, never declared complete with missing operations. The first functional milestone is P03, **not** the full release.

| Packet | Deliverable | Prerequisites |
| --- | --- | --- |
| [P00](00-foundation.md) | Tooling, offline CLI contract, catalog framework | Approved design |
| [P01](01-transport.md) | Connections, safe HTTP/output, PAT/external | P00 |
| [P02](02-oauth.md) | Persistent backends, user/bot OAuth | P01 |
| [P03](03-work-items.md) | Projects/states/labels/work-item lifecycle | P01, P02 |
| [P04](04-project-work.md) | Scheduling, relationships and collaboration | P03 |
| [P05](05-types-properties.md) | Types/custom properties/governance reads | P03 |
| [P06](06-content-files.md) | Knowledge/pages/assets/attachments | P03 |
| [P07](07-portfolio.md) | Portfolio/customers/releases/templates/views | P03, P05, P06 |
| [P08](08-administration.md) | Membership/permissions/webhooks/admin | P03, P05 |
| [P09](09-automation-legacy.md) | Workflows/automations/v1 supplemental gaps | P04, P05, P08 |
| [P10](10-delivery.md) | Full reconciliation, packaging, final review | P00–P09 |

P04/P05/P06 can proceed independently after shared P03 contracts are reviewed. P07/P08 can overlap after their prerequisites. Each worker owns disjoint domain files/catalog shards; one coordinator integrates shared root registration/catalog generation. Do not use parallel writers on the same registry, go.mod, Makefile or fixtures. Tool/dependency changes stay with the integration owner.

## Before each packet

Read the root agent instructions, the packet, and only its linked contracts. Filter [the source index](../reference/api-index.tsv) by named version/group; fetch the exact official operation pages. Record schema/docs provenance in `api/sources/`/`api/provenance.json`; exclude credentials/private tenant payloads. Reconcile every action, not just CRUD. A broad overview or migration table is not enough for a request fixture.

The coordinator freezes operation IDs, command mappings and shared contracts before delegating mechanical implementation. A worker may research within its family but must escalate evidence conflicts, missing public endpoints or required contract changes; it cannot drop scope or invent an API. P00 owns metadata format. If shards are needed, use one shard per reviewed domain with deterministic consolidation into the offline catalog; do not introduce parallel metadata conventions.

## Every packet's acceptance

1. Implement exact version-specific request/response semantics, complete input schema/help and relevant table projection, using the established transport/output/auth policies.
2. Exercise consumer-observable success/error/safety behavior through the fake-server CLI harness. Add only meaningful edge regressions; do not assert registration counts or source text.
3. Reconcile its catalog/source coverage and run the narrow relevant scenario after edits; then the packet's selected gate once. Report failures honestly.
4. Update the focused implemented user guide/command reference and verification evidence. Contracts remain canonical; never copy them into each packet.
5. Return changed files, operation IDs, exact commands/results, source hashes, remaining limitations and issues. No publication, commits, remote mutations or extra cleanup unless authorized.

## Planner handoff template

```text
Contract: execution (decided mapping) or bounded investigation (named evidence gap)
Packet and exact source families:
Owned files and shared integration owner:
Canonical contract links and fixed operation IDs:
Required request/response fixtures and acceptance command:
Non-goals and stop/escalation conditions:
Return verbatim small command output; exact excerpts/provenance for large evidence.
Redact credentials/auth state as [REDACTED].
Escalate errors, plan mismatches and judgment calls; do not improvise.
```

The planner reviews the gates in [verification](../verification.md). A weaker model is not responsible for choosing a new auth policy, resolving source contradictions by guesswork, or redefining release completeness.
