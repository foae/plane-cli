# P04 — Scheduling and work-item collaboration

Prerequisite P03. Read [API design](../design/api.md), [CLI contract](../design/cli-contract.md) and the per-operation source pages.

## Source ownership

V1: `cycle,module,milestones,estimate,intake-issue,inbox-issue,issue-activity,issue-comment,link,work-item-relations,worklogs`. V2: `cycles,modules,milestones,estimates,estimate-points,intake-work-items,work-item-activities,work-item-comments,work-item-links,work-item-worklogs,work-item-subscribers,work-item-votes,recurring-work-items`. Any relation operation indexed under `work-items` is explicitly handed from P03, not omitted or double counted.

Resolve supplemental specialized searches, grouped counts and lite lists for these families before closing P04, except for the explicitly approved evidence-blocked v1 `cycles-lite/` and `modules-lite/` routes recorded in [API contract gaps](../reference/api-contract-gaps.md). P09 reconciles coverage; it does not defer this packet's prerequisite evidence.

## Implement

Add domain command constructors and matching version adapters/fixtures, one family at a time. Extract all CRUD, membership add/remove, transfer, archive, summary/report and recurring/scheduling actions actually documented. Distinguish parent identifier flags from target `--id`; require explicit confirmations for removals/transfers whose actual semantics are destructive.

Do not confuse estimate systems with estimate points, intake triage with work-item creation, or relation metadata with related work items. Preserve date/time/duration and recurrence formats exactly; no user-local timezone conversion or cron invention. Pagination/fields use established adapters, not a new generic list client.

## Acceptance

Each family gets exact success/error fixtures and body-presence edges. Transfers assert source/destination and explicit side effects. Worklogs distinguish zero duration from absence. Recurrence fixture proves documented schedule representation. Comments/links preserve text/HTML without executing it; activities may be read-only. Catalog reconciliation proves each source action accounted for. Update focused scheduling/collaboration recipes only after they work.

## Integration evidence — 2026-11-14

The packet implements 154 documented operations across scheduling, intake, recurrence, relations and collaboration, with native v1/v2 routes, offline schemas and deterministic HTTP fixtures. Deprecated inbox sources follow the approved intake migration policy; the two v1 lite-route evidence gaps remain explicit in the linked gap register. Catalog lifecycle remains `implemented`, not independently `verified`.

The default panel completed all five reviews (`20261114-232312-3259312`). Accepted fixes embed IANA timezone data, expose manual cursors on the four documented cursor-capable collaboration lists, remove unsupported item-limit and unusable bulk body-field flags, correct bulk-update side-effect metadata, and retain the authoritative inbox deprecation evidence. Votes retain their documented offset-only surface. Legacy inbox endpoints were not added: the user explicitly approved supported intake commands rather than obsolete wire routes.

The remaining isolated suggestions do not establish blocking defects: cycle create/update use the documented `date-time` representation, while conflicting upsert prose is already treated separately; handler assertions rely on validated required flags; duplicated subscriber checks, validation-test precision and command-name dispatch are maintenance suggestions, not changed contracts. Speculative membership confirmation changes were not made without an authoritative destructive contract.

`make generate`, the focused P04/pagination regression suite under `-race`, `make check PACKET=P04`, `make security` and `make cross` passed. Lint reported zero issues; security found no reachable vulnerabilities (two findings remain in required modules outside the call graph); cross-builds cover Linux/macOS amd64/arm64. A compiled static binary successfully validated `Europe/London` in a Bubblewrap filesystem containing no host zoneinfo or Go installation. That smoke used dry-run, not a live Plane instance.

Verification repaired an unsynchronized request counter in the existing pagination deadline test. The new cursor regression initially used invalid UUID fixtures and incorrectly included offset-only votes; the final cases exercise the four documented cursor-capable resources. Fake-server acceptance and the isolated smoke do not establish live edition compatibility.
