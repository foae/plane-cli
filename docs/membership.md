# Membership, invitations and group sync

These commands send one explicitly selected native API version; they never probe another version, infer permissions from roles, enable a feature, or send a real invitation as a preflight.

## Discover and read

```sh
plane-cli commands --api-version v2
plane-cli schema member project create --api-version v2
plane-cli member workspace list --api-version v2 --workspace WORKSPACE
plane-cli member project list --api-version v2 --workspace WORKSPACE --project PROJECT_UUID
plane-cli member project get --api-version v2 --workspace WORKSPACE --project ENG --id MEMBERSHIP_UUID
plane-cli member project role-distribution --api-version v2 --workspace WORKSPACE
plane-cli invitation list --api-version v2 --workspace WORKSPACE --accepted=false
```

`member workspace list` and `member project list/get` support both versions. V2 project-list specifically requires a project UUID; v2 project detail/create/update/delete also accept the project's bare identifier. Membership IDs and user IDs are different: `--id` addresses a membership, while native `member_id` identifies a user. Do not substitute them or translate v1 bodies automatically.

V1 project-list's documented nested roster is retained as nested JSON. Other v1 lists have bare-array samples and no documented pagination controls. V2 responses retain sparse fields, null legacy roles, expansions and native envelopes. Role slugs are open strings unless the particular operation explicitly declares an enum; they are not a client-side permission model.

V2 lists default to one offset page. Native filters, `--fields`, `--expand`, `--order-by`, `--per-page`, `--offset`, `--count`, and `--paginate` are available only where their operation documents them. The leaf pages omit the cursor request field, but the [shared v2 pagination contract](https://developers.plane.so/api-reference/v2/pagination.md) explicitly documents `--cursor`; that supplement also enables bounded `--all --max-pages`. Omission of `--paginate` keeps the upstream offset default. Cursor mode must use upstream-eligible ordering; a rejected ordering is not silently changed. No pagination parameter is inferred for v1's nested roster.

## Explicitly confirmed writes

Every membership, invitation and group-sync mutation requires `--yes`, including POST/PATCH. `--dry-run` validates locally without credentials or HTTP, and reports confirmation requirements without sending a body. JSON files are native request documents, not cross-version DTOs. Protect identity, contact and organizational information in files and command output.

```sh
# First inspect the selected version's body schema and a request plan.
plane-cli schema invitation create --api-version v2
plane-cli invitation create --api-version v2 --workspace WORKSPACE --body-file invitation.json --dry-run
# Sends an invitation email only when explicitly executed with --yes.
plane-cli invitation create --api-version v2 --workspace WORKSPACE --body-file invitation.json --yes
```

A v2 single invitation body can be `{"email":"person@example.invalid","role":"member","message":null}`. V1 uses an integer `role` (20, 15 or 5), not a slug, and does not document `message`. Invitation list/get/create/delete exist in both versions; update is v1-only; bulk-create is v2-only.

V2 bulk-create accepts `emails` (1–100 valid nonempty addresses), one role (`admin`, `member` or `guest`, upstream default `member`) and an optional nonnullable message. Invalid input rejects the entire request; the client neither chunks it nor resends it. Its native 201 response is an array of invitations actually created: existing pending invitations are skipped without resend, so the array can be shorter or empty. An invitation is not a membership; explicit invitation detail and roster reads can inspect subsequent state.

Project member create/update/delete are available in both versions, with v1 native `member` and integer `role` versus v2 `member_id` and role slug. V2 workspace removal uses `member workspace remove` with native `email` and optional `remove_seat`; deletion of a project membership is a different operation. Review seat/deactivation implications before confirming removal.

**V1 workspace removal is not callable.** Its [official page](https://developers.plane.so/api-reference/members/remove-workspace-member.md) omits every target-identifying request field, despite documenting deactivation across projects and seat-count reduction. No v2 payload, target lookup or empty-target stub is substituted.

## Group-sync configuration and mappings

```sh
plane-cli group-sync config get --api-version v2 --workspace WORKSPACE
# IMPORTANT: v1 GET can create missing configuration defaults.
plane-cli group-sync config get --api-version v1 --workspace WORKSPACE --dry-run
plane-cli group-sync config get --api-version v1 --workspace WORKSPACE --yes
plane-cli group-sync project-mapping list --api-version v1 --workspace WORKSPACE --project-identifier ENG
plane-cli group-sync workspace-mapping list --api-version v2 --workspace WORKSPACE
plane-cli schema group-sync config update --api-version v2
```

**V1 config GET is a write:** the first request can create configuration. It requires `--yes` and uses mutation transport classification despite HTTP GET and its documented read scope. No default values are inferred from samples. V2 config GET is an ordinary read and needs no confirmation. Offline command-level help conservatively labels config get as write; its summary explains the version difference.

Both versions support config get/update and workspace-/project-mapping list/get/create/update/delete. These manage mapping metadata, not an invented immediate sync execution. V1 configuration uses `default_workspace_role`, mappings use `role` and `project`; v2 uses nullable `default_workspace_role_slug`, `role_slug`, and nullable UUID `project_id`. Preserve those distinctions. V1 project and `all_projects` target fields are mutually exclusive. Its mapping updates reject an empty object. No project/role defaults are inserted by the CLI.

V1 additionally supports exact by-key addressing:

```sh
plane-cli group-sync project-mapping update-by-key --api-version v1 --workspace WORKSPACE --project-key ENG --idp-group-name 'Exact IdP group name' --body-file mapping-update.json --dry-run
```

`--project-key` is matched case-insensitively upstream; the group name is exact and path-escaped as one segment. A body's `idp_group_name` can rename the mapping independently of the addressing key. By-key lookup is project-scoped; use `--id` for all-project mappings. V2 has no evidenced by-key operation. Group-sync administration requires the upstream permissions and scopes; the CLI does not grant them.

## Evidence and failures

The [v2 members overview](https://developers.plane.so/api-reference/v2/members/overview.md) contradicts itself: its endpoint table lists writes, while its migration paragraph says reads only. Dedicated create/update/delete/remove operation pages take precedence, with this contradiction retained in source evidence. The v1 nested roster and missing removal target remain explicit upstream gaps.

402 denotes feature/plan entitlement, 403 authorization, and 404 a missing or hidden/out-of-scope resource—not proof of edition. Responses do not trigger role inference, retries, alternate endpoints or version fallback. Fake-server request fixtures establish the implemented wire contract, not deployed Plane compatibility or real invitation delivery.
