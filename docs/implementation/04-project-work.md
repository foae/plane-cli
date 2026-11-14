# P04 — Scheduling and work-item collaboration

Prerequisite P03. Read [API design](../design/api.md), [CLI contract](../design/cli-contract.md) and the per-operation source pages.

## Source ownership

V1: `cycle,module,milestones,estimate,intake-issue,inbox-issue,issue-activity,issue-comment,link,work-item-relations,worklogs`. V2: `cycles,modules,milestones,estimates,estimate-points,intake-work-items,work-item-activities,work-item-comments,work-item-links,work-item-worklogs,work-item-subscribers,work-item-votes,recurring-work-items`. Any relation operation indexed under `work-items` is explicitly handed from P03, not omitted or double counted.

Resolve supplemental specialized searches, grouped counts and lite lists for these families before closing P04. P09 reconciles coverage; it does not defer this packet's prerequisite evidence.

## Implement

Add domain command constructors and matching version adapters/fixtures, one family at a time. Extract all CRUD, membership add/remove, transfer, archive, summary/report and recurring/scheduling actions actually documented. Distinguish parent identifier flags from target `--id`; require explicit confirmations for removals/transfers whose actual semantics are destructive.

Do not confuse estimate systems with estimate points, intake triage with work-item creation, or relation metadata with related work items. Preserve date/time/duration and recurrence formats exactly; no user-local timezone conversion or cron invention. Pagination/fields use established adapters, not a new generic list client.

## Acceptance

Each family gets exact success/error fixtures and body-presence edges. Transfers assert source/destination and explicit side effects. Worklogs distinguish zero duration from absence. Recurrence fixture proves documented schedule representation. Comments/links preserve text/HTML without executing it; activities may be read-only. Catalog reconciliation proves each source action accounted for. Update focused scheduling/collaboration recipes only after they work.
