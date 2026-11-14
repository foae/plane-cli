# P03 — First complete work-item lifecycle

Prerequisites P01/P02. Read [API design](../design/api.md), [CLI contract](../design/cli-contract.md), [verification](../verification.md). Source groups: v1 `user,project,state,label,project-labels,issue`; v2 `users,projects,states,workspace-states,labels,project-states,project-labels,work-items`. P03 owns identity reads, project lifecycle and every state/label definition/assignment action in these groups; P08 owns user administration and separately indexed feature/member/permission operations, not state/label CRUD. Workflow states/transitions are P09. Record each exact operation under one owner before handoff.

## Own

CLI `project.go`, `state.go`, `label.go`, `work_item.go`; matching `internal/plane/v1/` and `v2/` domain files; version-specific pagination helpers where first used; domain schemas, fixtures, table columns and source evidence. Keep root registration/catalog integration with the coordinator.

## Ordered work

1. Resolve exact per-version identity, project, state/label and work-item mappings, including supplemental specialized search/grouped-count/lite-list evidence now, not in P09. Lifecycle actions—including native bulk/upsert/archive/search/identifier reads—must all be mapped or explicitly blocked by evidence. Do not drop non-CRUD operations because the first walkthrough is small.
2. Implement identity/projects first, then states/labels, then work items. Common flags plus complete JSON bodies must cover the full documented writable schema with presence/null behavior. Version-native fields remain unchanged in output.
3. Implement one-page and bounded `--all` native pagination, field/expand/filter flags, explicit scopes/version validation, dry-run and destructive guards. Implement declared table projections without relation lookups.
4. Prove real create → get → update → list → delete against separate v1/v2 fake-server scripts. Include UUID/identifier distinctions, sparse responses, feature/permission errors and no implicit version fallback. Require accepted/partial response fixtures only for operations whose exact documented contracts support them; shared transport tests cover generic 202/partial handling without fabricating Plane behavior.
5. Publish the first working getting-started guide: configure connection/API generation/auth, discover schema, select project/state/labels, create/update/read/delete. Use fake placeholders, never real credentials. Document JSON nesting and empty/null behavior.

## Acceptance

Exact request fixtures for every owned operation plus lifecycle process tests; omission/null/zero/false/empty distinctions; offset/cursor and real v1 pagination; large integers/unknown fields; staged --all failure; native mixed bulk results where documented; mutation timeout with unknown outcome; no request without required --yes. Run focused domain suite plus catalog reconciliation and `make check` at integration.

Planner reviews this completed vertical slice before copying its patterns to the remaining families. P03 is usable core functionality, not permission to ship a “full API” release with P04–P09 missing.
