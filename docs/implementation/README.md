# Implementation packet router

P00–P02 provide offline discovery, transport and authentication. P03 implements the documented identity, project, state/label and work-item operations; its integration and planner evidence determine completion. Complete packets in dependency order; a packet may be divided into source-family slices, never declared complete with missing operations. The first API milestone is P03, **not** the full release. Shared security contracts require planner review before dependent resource work.

| Packet | Deliverable | Prerequisites |
| --- | --- | --- |
| [P00](00-foundation.md) | Tooling, offline CLI contract, catalog framework | Approved design |
| [P01](01-transport.md) | Connections, safe HTTP/output, PAT/external | P00 |
| [P02](02-oauth.md) | Persistent backends, user/bot OAuth | P01 |
| [P03](03-work-items.md) | Projects, all state/label definitions, work-item lifecycle/search | P01, P02 |
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

The coordinator freezes exact operation IDs, command mappings and shared contracts before mechanical implementation. A worker may research within its family but must escalate evidence conflicts, missing public endpoints or required contract changes; it cannot drop scope or invent an API. P00 owns metadata format/source ledger; each domain owns its exact evidence extraction before implementation. Use one reviewed domain shard when needed, deterministically consolidated by the coordinator; never parallel metadata conventions. P03/P04 supplemental search and P06 upload evidence are dependencies of those packets, not late P09 work.

Before assigning a broad packet, instantiate the handoff below for one source family/version pair or a documented multi-phase workflow. Fill actual operation IDs, concrete CLI/adapter/schema/fixture paths, exact acceptance command and evidence links; an unfilled packet document is not an execution handoff. Keep v1/v2 variants together only when the bounded slice and its fixtures remain explicit. The coordinator owns shared registration, generated metadata and dependency files. Every operation has exactly one packet and slice owner; source-page aliases do not create duplicate owners. Unresolved evidence produces a bounded investigation handoff, not permission for a cheaper model to choose the contract.

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
