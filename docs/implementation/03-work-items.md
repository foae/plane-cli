# P03 — First complete work-item lifecycle

Prerequisites P01/P02. Read [API design](../design/api.md), [CLI contract](../design/cli-contract.md), [verification](../verification.md). Source groups: v1 `user,project,state,label,project-labels,issue,epics`; v2 `users,projects,states,workspace-states,labels,project-states,project-labels,work-items`. P03 owns identity reads, project lifecycle and every state/label definition/assignment action in these groups; P08 owns user administration and separately indexed feature/member/permission operations, not state/label CRUD. Workflow states/transitions are P09. Record each exact operation under one owner before handoff.

V1 coverage is best-effort. Supplemental grouped counts, additional search and lite-list operations whose exact official contracts are unavailable remain explicitly evidence-blocked; they do not block delivery of independently documented P03 operations. Do not infer their methods, paths or schemas from migration mentions. This exception does not weaken v2 acceptance or permit claiming complete public API coverage while known gaps remain.

Track unresolved evidence in [API contract gaps](../reference/api-contract-gaps.md); continue independently documented operations rather than stopping the packet for best-effort v1 gaps.

## Own

CLI `project.go`, `state.go`, `label.go`, `work_item.go`, `epic.go`; version-specific pagination helpers where first used; domain schemas, fixtures, table columns and source evidence. Transparent native JSON transport avoids redundant version DTO packages. Keep root registration/catalog integration with the coordinator.

## Ordered work

1. Resolve exact per-version identity, project, state/label and work-item mappings, including supplemental specialized search/grouped-count/lite-list evidence now, not in P09. Lifecycle actions—including native bulk/upsert/archive/search/identifier reads—must all be mapped or explicitly blocked by evidence. Do not drop non-CRUD operations because the first walkthrough is small.
2. Implement identity/projects first, then states/labels, then work items. Common flags plus complete JSON bodies must cover the full documented writable schema with presence/null behavior. Version-native fields remain unchanged in output.
3. Implement one-page and bounded `--all` native pagination, field/expand/filter flags, explicit scopes/version validation, dry-run and destructive guards. Implement declared table projections without relation lookups.
4. Prove real create → get → update → list → delete against separate v1/v2 fake-server scripts. Include UUID/identifier distinctions, sparse responses, feature/permission errors and no implicit version fallback. Require accepted/partial response fixtures only for operations whose exact documented contracts support them; shared transport tests cover generic 202/partial handling without fabricating Plane behavior.
5. Publish the first working getting-started guide: configure connection/API generation/auth, discover schema, select project/state/labels, create/update/read/delete. Use fake placeholders, never real credentials. Document JSON nesting and empty/null behavior.

## Acceptance

Exact request fixtures for every owned operation plus lifecycle process tests; omission/null/zero/false/empty distinctions; offset/cursor and real v1 pagination; large integers/unknown fields; staged --all failure; native mixed bulk results where documented; mutation timeout with unknown outcome; no request without required --yes. Run focused domain suite plus catalog reconciliation and `make check` at integration.

Planner reviews this completed vertical slice before copying its patterns to the remaining families. P03 is usable core functionality, not permission to ship a “full API” release with P04–P09 missing.

## Integration evidence — 2026-11-14

Implemented documented identity, project, state/label, work-item and v1 epic operations with embedded discovery/schema metadata and fake-server acceptance. The default five-model review identified nine accepted findings; repairs cover project-upsert fields, strict pagination continuations, invocation-wide stdin deadlines, aggregate deadline classification/progress, read-only POST error semantics, negative pagination scenarios, bulk-update 409 metadata and acceptance-reference validation. Source reconciliation also records project-upsert 201 creation responses.

After correcting two integration-test setup errors (offline fixture connection settings and an unsupported v1 filter flag), `make generate` and `make check PACKET=P03` passed: formatting, lint (zero issues), Go tests, module consistency, catalog reconciliation and offline documentation/schema validation. Fixtures prove CLI protocol behavior, not live Plane edition compatibility.

The user authorized delivery without another review round and stopping after P03. Operation lifecycle remains `implemented`, not independently `verified`; the checker reports zero verified operations. Supplemental v1 evidence gaps remain explicit in the linked gap register. P04 and later packets are not part of this delivery.

## Optional live acceptance — 2026-11-14

The compiled CLI was run inside the disposable Incus container `plane-cli-e2e`, against Plane Community `v1.4.3` using the published Docker Compose stack. Backend image digest: `sha256:9b75bf46cf4ffa94272a337450ab96a4577f2b81ceff00def11f5c71eb8dd743`. A database bootstrap created only the disposable user/workspace/API key; lifecycle operations used the CLI and real HTTP endpoints.

Observed passing v1 operations: API-key file login/status/logout, identity, project create/get/update/list/archive/unarchive/delete, project-scoped state and label CRUD/list, work-item create/get/update/list/delete, identifier lookup and native search. Read-back assertions checked identity, created name, assigned state/label and cleared labels after update. JSON and table output were exercised. Missing `--yes` returned exit 2 and preserved the work item; a subsequent confirmed delete produced a 404 on read. Logout produced exit 3 on the next authenticated command. `--all` succeeded on the small live collections; this is not evidence of multi-page traversal.

Deployment limits: advanced search returned HTTP 403, while epic list, workspace project-label list and v2 identity returned HTTP 404. Those responses do not establish whether other editions expose these operations. This run does not prove all P03 operations, OAuth providers, native keyrings, v2 compatibility or full API coverage, and does not promote catalog lifecycle status.

Sandbox setup required dedicated `DOCKER-USER` rules permitting outbound forwarding from `planeclitest` and established return traffic; Docker's host forwarding policy otherwise blocked package downloads. Keep the published nonempty `CERT_ACME_CA` URL even with `SITE_ADDRESS=:80`, because an empty value prevents Caddy parsing. Wait for API readiness after migrations/recreation: a running proxy can temporarily return 502. These are deployment prerequisites, not CLI repairs.

The sandbox is on a dedicated NAT bridge (`10.119.0.0/24`); no host public port is published. Credentials remain owner-only inside the container. Remove the bridge-specific forwarding rules when tearing down the container and bridge; do not change the host's default forwarding policy.

Commercial `v4.0.4` was subsequently installed in the same disposable container with the official Prime installer (backend digest `sha256:b1d878173d175a9592e8905c77ff713ab8269354effa02ace64bdba2c9f718c2`). The installer completed migrations and API readiness. The v1 lifecycle exercised create/read/update/list, identifier lookup, native search, confirmed deletion and project archive/unarchive; advanced search and workspace project-label listing succeeded on this edition. Epic listing returned HTTP 402, leaving epic lifecycle acceptance blocked by the deployment's entitlement.

Commercial v1 pagination assertions created three work items, requested one item per page, and verified that `--all` returned exactly three native page envelopes containing the three IDs. `--max-pages 1` returned `page_limit`, exit 1 and incomplete progress. Dry-run create left the collection unchanged; invalid priority returned exit 2. The disposable runner initially expected the wrong page-limit exit and assumed unexpanded relations; its assertions were corrected for the existing CLI contract and Commercial's native relation objects, without changing CLI output.

Commercial v2 checks passed for identity; project create/get/update/list/summary/delete; project-scoped state and label CRUD/list; work-item create/get/update/list/delete, workspace count, bulk update, archive/unarchive and upsert. Read-back assertions checked the work-item name after creation/update and priority after bulk update. These are real CLI/HTTP checks, not fixture results. They are bounded evidence, not exhaustive acceptance of all bulk/upsert variants, workspace states, project-state/project-label mutations, v2 pagination, OAuth or keyring behavior.

A burst of live requests reached HTTP 429 with `retry_after_seconds: 22`; the CLI exposed the rejection and did not silently retry the mutation. Subsequent requests were paced and the rejected deletion completed successfully. No CLI implementation defect was established by these live runs. Deployment availability and licensing remain distinct from documented API support; catalog lifecycle status is unchanged.
