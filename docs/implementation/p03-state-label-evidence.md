# P03 state and label implementation evidence

Collected 2026-11-14. All 57 owned indexed source pages were read (49 operation pages, 8 overviews). Descriptors: `api/catalog.json` (29 commands; 44 primary version mappings and 5 explicit workspace-state alternate routes). Handlers: `internal/cli/state.go`, `internal/cli/label.go`; factory `stateLabelHandlers()`. No version DTO package added: transparent RawMessage transport preserves native keys and unknown fields without redundant wire models.

## Contract decisions and gaps

- State CRUD requires explicit `--scope project|workspace`; v1 and project-only upsert/bulk require `--scope project`. Configured project never selects scope. v2 scope routes are recorded in Variant `routes`; conditional schemas require project for project scope and enforce each workspace/project write schema independently.
- Workspace catalog states disallow `triage` and `is_default`; workspace create requires `group`. Project lifecycle state groups are separate (`draft`, `planning`, `execution`, `monitoring`, `completed`, `cancelled`). v1 state operation parameters explicitly allow `triage` and writable `is_triage`, unlike the narrower overview; operation definitions control validation.
- Upsert contradiction, resolved by Main: both operation introductions require `external_source` and `external_id`; ApiParam metadata calls them optional/nullable. Upsert alone requires nonnull strings, without invented nonempty validation. Create/update keep their independent nullability.
- Exact upsert excerpt (both v2 states/upsert-state.md and labels/upsert-label.md): “Both `external_source` and `external_id` are required. Without them there is nothing to match on, and the request returns `400`.” ApiParam excerpt: `<ApiParam name="external_id" type="string" :required="false">` followed by “Nullable.”
- Exact v2 project-label overview: “The v2 API manages the catalog: list, read, create, rename, recolor, reorder, and delete. It has no endpoint or project field that attaches a label to a project.” No invented attach action. Label assignment is work-item `label_ids` (owned by work-item implementation); project lifecycle state assignment is project `state_id` (coordinator project implementation). The independently indexed assignment action here is `project-state mark-default`, a bodyless POST.
- Native bulk inputs: `items` for create/update, `ids` for delete; 1–50 rows, UUIDs for update/delete identities, optional `all_or_none`. No chunking, retries, v1 bulk emulation, or upsert concurrency claims. Bulk 200 native `failed>0` classification is coordinator-owned in apiRequest; handlers pass the complete envelope through.
- Exact bulk response excerpt: `{"results":[{"index":0,"result":"updated","id":"..."},{"index":1,"result":"failed","type":"not_found","code":"not_found","detail":"..."}],"succeeded":1,"failed":1}`. All-or-none failures are documented HTTP 409, not partial success.
- Every v1 list here documents `results`, `next_cursor`, `next_page_results`; no bare-array exception. Every v2 list documents offset/cursor envelopes. v2 label reads explicitly have no fields/expand. v1 label introductory prose mentions name/color filtering, but the operation query parameter list defines neither filter name nor contract; no speculative v1 filter parameters are exposed. This v1-only evidence gap does not block operations.
- Workspace state list accepts project_id only as a filter (`--filter-project`), distinct from default/path `--project`; project scope rejects this filter. Workspace state list does not document is_default filtering; project scope alone permits it.
- Generic 402/403/404/409 and governance failures remain upstream errors without preflight/fallback. No state transfer/workflow actions are invented from P09 migration references.

## Operation mappings

| Version | Source group / slug | Command | Scope | Method | Exact path | Success status | Pagination |
|---|---|---|---|---|---|---|---|
| v2 | [states/list-states](https://developers.plane.so/api-reference/v2/states/list-states.md) | `state list` | project | GET | `/api/v2/workspaces/{slug}/projects/{project_id}/states/` | 200 | v2 |
| v2 | [states/get-state](https://developers.plane.so/api-reference/v2/states/get-state.md) | `state get` | project | GET | `/api/v2/workspaces/{slug}/projects/{project_id}/states/{pk}/` | 200 | none |
| v2 | [states/create-state](https://developers.plane.so/api-reference/v2/states/create-state.md) | `state create` | project | POST | `/api/v2/workspaces/{slug}/projects/{project_id}/states/` | 201 | none |
| v2 | [states/update-state](https://developers.plane.so/api-reference/v2/states/update-state.md) | `state update` | project | PATCH | `/api/v2/workspaces/{slug}/projects/{project_id}/states/{pk}/` | 200 | none |
| v2 | [states/delete-state](https://developers.plane.so/api-reference/v2/states/delete-state.md) | `state delete` | project | DELETE | `/api/v2/workspaces/{slug}/projects/{project_id}/states/{pk}/` | 204 | none |
| v2 | [states/upsert-state](https://developers.plane.so/api-reference/v2/states/upsert-state.md) | `state upsert` | project | POST | `/api/v2/workspaces/{slug}/projects/{project_id}/states/upsert/` | 200 | none |
| v2 | [states/bulk-create-states](https://developers.plane.so/api-reference/v2/states/bulk-create-states.md) | `state bulk-create` | project | POST | `/api/v2/workspaces/{slug}/projects/{project_id}/states/bulk-create/` | 200 | none |
| v2 | [states/bulk-update-states](https://developers.plane.so/api-reference/v2/states/bulk-update-states.md) | `state bulk-update` | project | POST | `/api/v2/workspaces/{slug}/projects/{project_id}/states/bulk-update/` | 200 | none |
| v2 | [states/bulk-delete-states](https://developers.plane.so/api-reference/v2/states/bulk-delete-states.md) | `state bulk-delete` | project | POST | `/api/v2/workspaces/{slug}/projects/{project_id}/states/bulk-delete/` | 200 | none |
| v2 | [workspace-states/list-workspace-states](https://developers.plane.so/api-reference/v2/workspace-states/list-workspace-states.md) | `state list` | workspace | GET | `/api/v2/workspaces/{slug}/states/` | 200 | v2 |
| v2 | [workspace-states/get-workspace-state](https://developers.plane.so/api-reference/v2/workspace-states/get-workspace-state.md) | `state get` | workspace | GET | `/api/v2/workspaces/{slug}/states/{pk}/` | 200 | none |
| v2 | [workspace-states/create-workspace-state](https://developers.plane.so/api-reference/v2/workspace-states/create-workspace-state.md) | `state create` | workspace | POST | `/api/v2/workspaces/{slug}/states/` | 201 | none |
| v2 | [workspace-states/update-workspace-state](https://developers.plane.so/api-reference/v2/workspace-states/update-workspace-state.md) | `state update` | workspace | PATCH | `/api/v2/workspaces/{slug}/states/{pk}/` | 200 | none |
| v2 | [workspace-states/delete-workspace-state](https://developers.plane.so/api-reference/v2/workspace-states/delete-workspace-state.md) | `state delete` | workspace | DELETE | `/api/v2/workspaces/{slug}/states/{pk}/` | 204 | none |
| v2 | [labels/list-labels](https://developers.plane.so/api-reference/v2/labels/list-labels.md) | `label list` | project | GET | `/api/v2/workspaces/{slug}/projects/{project_id}/labels/` | 200 | v2 |
| v2 | [labels/get-label](https://developers.plane.so/api-reference/v2/labels/get-label.md) | `label get` | project | GET | `/api/v2/workspaces/{slug}/projects/{project_id}/labels/{pk}/` | 200 | none |
| v2 | [labels/create-label](https://developers.plane.so/api-reference/v2/labels/create-label.md) | `label create` | project | POST | `/api/v2/workspaces/{slug}/projects/{project_id}/labels/` | 201 | none |
| v2 | [labels/update-label](https://developers.plane.so/api-reference/v2/labels/update-label.md) | `label update` | project | PATCH | `/api/v2/workspaces/{slug}/projects/{project_id}/labels/{pk}/` | 200 | none |
| v2 | [labels/delete-label](https://developers.plane.so/api-reference/v2/labels/delete-label.md) | `label delete` | project | DELETE | `/api/v2/workspaces/{slug}/projects/{project_id}/labels/{pk}/` | 204 | none |
| v2 | [labels/upsert-label](https://developers.plane.so/api-reference/v2/labels/upsert-label.md) | `label upsert` | project | POST | `/api/v2/workspaces/{slug}/projects/{project_id}/labels/upsert/` | 200 | none |
| v2 | [labels/bulk-create-labels](https://developers.plane.so/api-reference/v2/labels/bulk-create-labels.md) | `label bulk-create` | project | POST | `/api/v2/workspaces/{slug}/projects/{project_id}/labels/bulk-create/` | 200 | none |
| v2 | [labels/bulk-update-labels](https://developers.plane.so/api-reference/v2/labels/bulk-update-labels.md) | `label bulk-update` | project | POST | `/api/v2/workspaces/{slug}/projects/{project_id}/labels/bulk-update/` | 200 | none |
| v2 | [labels/bulk-delete-labels](https://developers.plane.so/api-reference/v2/labels/bulk-delete-labels.md) | `label bulk-delete` | project | POST | `/api/v2/workspaces/{slug}/projects/{project_id}/labels/bulk-delete/` | 200 | none |
| v2 | [project-states/list-project-states](https://developers.plane.so/api-reference/v2/project-states/list-project-states.md) | `project-state list` | workspace catalog | GET | `/api/v2/workspaces/{slug}/project-states/` | 200 | v2 |
| v2 | [project-states/get-project-state](https://developers.plane.so/api-reference/v2/project-states/get-project-state.md) | `project-state get` | workspace catalog | GET | `/api/v2/workspaces/{slug}/project-states/{pk}/` | 200 | none |
| v2 | [project-states/create-project-state](https://developers.plane.so/api-reference/v2/project-states/create-project-state.md) | `project-state create` | workspace catalog | POST | `/api/v2/workspaces/{slug}/project-states/` | 201 | none |
| v2 | [project-states/update-project-state](https://developers.plane.so/api-reference/v2/project-states/update-project-state.md) | `project-state update` | workspace catalog | PATCH | `/api/v2/workspaces/{slug}/project-states/{pk}/` | 200 | none |
| v2 | [project-states/delete-project-state](https://developers.plane.so/api-reference/v2/project-states/delete-project-state.md) | `project-state delete` | workspace catalog | DELETE | `/api/v2/workspaces/{slug}/project-states/{pk}/` | 204 | none |
| v2 | [project-states/mark-default-project-state](https://developers.plane.so/api-reference/v2/project-states/mark-default-project-state.md) | `project-state mark-default` | workspace catalog | POST | `/api/v2/workspaces/{slug}/project-states/{pk}/mark-default/` | 200 | none |
| v2 | [project-labels/list-project-labels](https://developers.plane.so/api-reference/v2/project-labels/list-project-labels.md) | `project-label list` | workspace catalog | GET | `/api/v2/workspaces/{slug}/project-labels/` | 200 | v2 |
| v2 | [project-labels/get-project-label](https://developers.plane.so/api-reference/v2/project-labels/get-project-label.md) | `project-label get` | workspace catalog | GET | `/api/v2/workspaces/{slug}/project-labels/{pk}/` | 200 | none |
| v2 | [project-labels/create-project-label](https://developers.plane.so/api-reference/v2/project-labels/create-project-label.md) | `project-label create` | workspace catalog | POST | `/api/v2/workspaces/{slug}/project-labels/` | 201 | none |
| v2 | [project-labels/update-project-label](https://developers.plane.so/api-reference/v2/project-labels/update-project-label.md) | `project-label update` | workspace catalog | PATCH | `/api/v2/workspaces/{slug}/project-labels/{pk}/` | 200 | none |
| v2 | [project-labels/delete-project-label](https://developers.plane.so/api-reference/v2/project-labels/delete-project-label.md) | `project-label delete` | workspace catalog | DELETE | `/api/v2/workspaces/{slug}/project-labels/{pk}/` | 204 | none |
| v1 | [project-labels/add-project-label](https://developers.plane.so/api-reference/project-labels/add-project-label.md) | `project-label create` | workspace catalog | POST | `/api/v1/workspaces/{workspace_slug}/project-labels/` | 201 | none |
| v1 | [project-labels/list-project-labels](https://developers.plane.so/api-reference/project-labels/list-project-labels.md) | `project-label list` | workspace catalog | GET | `/api/v1/workspaces/{workspace_slug}/project-labels/` | 200 | v1 |
| v1 | [project-labels/get-project-label-detail](https://developers.plane.so/api-reference/project-labels/get-project-label-detail.md) | `project-label get` | workspace catalog | GET | `/api/v1/workspaces/{workspace_slug}/project-labels/{label_id}/` | 200 | none |
| v1 | [project-labels/update-project-label-detail](https://developers.plane.so/api-reference/project-labels/update-project-label-detail.md) | `project-label update` | workspace catalog | PATCH | `/api/v1/workspaces/{workspace_slug}/project-labels/{label_id}/` | 200 | none |
| v1 | [project-labels/delete-project-label](https://developers.plane.so/api-reference/project-labels/delete-project-label.md) | `project-label delete` | workspace catalog | DELETE | `/api/v1/workspaces/{workspace_slug}/project-labels/{label_id}/` | 204 | none |
| v1 | [state/add-state](https://developers.plane.so/api-reference/state/add-state.md) | `state create` | project | POST | `/api/v1/workspaces/{workspace_slug}/projects/{project_id}/states/` | 200 | none |
| v1 | [state/list-states](https://developers.plane.so/api-reference/state/list-states.md) | `state list` | project | GET | `/api/v1/workspaces/{workspace_slug}/projects/{project_id}/states/` | 200 | v1 |
| v1 | [state/get-state-detail](https://developers.plane.so/api-reference/state/get-state-detail.md) | `state get` | project | GET | `/api/v1/workspaces/{workspace_slug}/projects/{project_id}/states/{state_id}/` | 200 | none |
| v1 | [state/update-state-detail](https://developers.plane.so/api-reference/state/update-state-detail.md) | `state update` | project | PATCH | `/api/v1/workspaces/{workspace_slug}/projects/{project_id}/states/{state_id}/` | 200 | none |
| v1 | [state/delete-state](https://developers.plane.so/api-reference/state/delete-state.md) | `state delete` | project | DELETE | `/api/v1/workspaces/{workspace_slug}/projects/{project_id}/states/{state_id}/` | 204 | none |
| v1 | [label/add-label](https://developers.plane.so/api-reference/label/add-label.md) | `label create` | project | POST | `/api/v1/workspaces/{workspace_slug}/projects/{project_id}/labels/` | 201 | none |
| v1 | [label/list-labels](https://developers.plane.so/api-reference/label/list-labels.md) | `label list` | project | GET | `/api/v1/workspaces/{workspace_slug}/projects/{project_id}/labels/` | 200 | v1 |
| v1 | [label/get-label-detail](https://developers.plane.so/api-reference/label/get-label-detail.md) | `label get` | project | GET | `/api/v1/workspaces/{workspace_slug}/projects/{project_id}/labels/{label_id}/` | 200 | none |
| v1 | [label/update-label-detail](https://developers.plane.so/api-reference/label/update-label-detail.md) | `label update` | project | PATCH | `/api/v1/workspaces/{workspace_slug}/projects/{project_id}/labels/{label_id}/` | 200 | none |
| v1 | [label/delete-label](https://developers.plane.so/api-reference/label/delete-label.md) | `label delete` | project | DELETE | `/api/v1/workspaces/{workspace_slug}/projects/{project_id}/labels/{label_id}/` | 204 | none |

## Snapshot provenance

SHA-256 below covers the exact UTF-8 markdown returned by `read(URL:1-:raw)`; no reader truncation markers or reconstructed excerpts. Direct urllib retrieval returned HTTP 403; official documents were available through read. Redistribution permission was not established, so full source pages are not vendored: only URLs, hashes, minimal excerpts and independently expressed schemas are retained.

| Source | SHA-256 | UTF-8 bytes | Classification |
|---|---|---|---|
| [v2/states/overview](https://developers.plane.so/api-reference/v2/states/overview.md) | `853bef0fea1d80f14372acbf63a3aa0e5655bec5cdf8337da1e9aea61f3b2544` | 5958 | guide |
| [v2/states/list-states](https://developers.plane.so/api-reference/v2/states/list-states.md) | `0b4895596cb8bc01fa5ed5349c5b8377cfb2a0c5d17740444ac8b1cdd8083404` | 10707 | operation |
| [v2/states/get-state](https://developers.plane.so/api-reference/v2/states/get-state.md) | `a998f350a856ab51734ec0ec91fa7c599f93d7b8e0363b22826b4a4c17f8d55a` | 5529 | operation |
| [v2/states/create-state](https://developers.plane.so/api-reference/v2/states/create-state.md) | `050ecde8cb23e802cc68f4ca0ae80e986bb3f77650b9125c84b51a8afac5d848` | 8597 | operation |
| [v2/states/update-state](https://developers.plane.so/api-reference/v2/states/update-state.md) | `9abd8aefc210ffd71faff9f575849bb1b17ec7bf3dd22db32915226929834e76` | 9778 | operation |
| [v2/states/delete-state](https://developers.plane.so/api-reference/v2/states/delete-state.md) | `746a441d4ba05580dac0b54bc81d3273eb8658a48d9f2d93f256ccd382d3cd26` | 7617 | operation |
| [v2/states/upsert-state](https://developers.plane.so/api-reference/v2/states/upsert-state.md) | `9c138118b7a6278a846937a4aa94882e7ce2a2acb3c5d63ebc7c02354af4e0bd` | 8502 | operation |
| [v2/states/bulk-create-states](https://developers.plane.so/api-reference/v2/states/bulk-create-states.md) | `bd1b8b8da1006a7fe4c2a6aa8fb320b53dd87d412710fb2041c80b1c89220778` | 8720 | operation |
| [v2/states/bulk-update-states](https://developers.plane.so/api-reference/v2/states/bulk-update-states.md) | `cbef07d3cd50265c15ca52df58f76458e2202d7f3b7706c58b74264abcf2288e` | 8624 | operation |
| [v2/states/bulk-delete-states](https://developers.plane.so/api-reference/v2/states/bulk-delete-states.md) | `1a9511f8422ad4cce6f5ed3d692655c387e7f5416f9912f89c1926b7e04e0554` | 8102 | operation |
| [v2/workspace-states/overview](https://developers.plane.so/api-reference/v2/workspace-states/overview.md) | `7e378c29446225afc6efc9c236f9048cedb1aaaab8c27b38050973614bb48637` | 8649 | guide |
| [v2/workspace-states/list-workspace-states](https://developers.plane.so/api-reference/v2/workspace-states/list-workspace-states.md) | `ff238f28c1ea5b5845ad64afe84dbf885418fe449eb476fcaef4778b42e1dd0a` | 10455 | operation |
| [v2/workspace-states/get-workspace-state](https://developers.plane.so/api-reference/v2/workspace-states/get-workspace-state.md) | `e684c474bc843bf9b70f78b958eafc0e8da70292daec6c53c0a94ddfe281a054` | 4873 | operation |
| [v2/workspace-states/create-workspace-state](https://developers.plane.so/api-reference/v2/workspace-states/create-workspace-state.md) | `2f58b8541f24fd8822a7a072457c0386e227f9f836b7819e3afc547b7de032a0` | 9133 | operation |
| [v2/workspace-states/update-workspace-state](https://developers.plane.so/api-reference/v2/workspace-states/update-workspace-state.md) | `a894d0f5fb3b138a444a0dc1fcf5b30bf7d17287ca7853ad1838e23a15ce4c24` | 8528 | operation |
| [v2/workspace-states/delete-workspace-state](https://developers.plane.so/api-reference/v2/workspace-states/delete-workspace-state.md) | `676c3af4af9f8f5aba6014166fba9d22c5e90ffdf7dd2ef6fb58d87c6aa9a306` | 6716 | operation |
| [v2/labels/overview](https://developers.plane.so/api-reference/v2/labels/overview.md) | `c523a6cebb88cfded0d59fac3b27e8162ec229792247a84578b8ca1efc8d7dd6` | 7203 | guide |
| [v2/labels/list-labels](https://developers.plane.so/api-reference/v2/labels/list-labels.md) | `d383cb76f543882083d013e0f28d2de4ca2e6c3559dc995b97f172b14edd1c0d` | 8731 | operation |
| [v2/labels/get-label](https://developers.plane.so/api-reference/v2/labels/get-label.md) | `5e72c46d39c29c3173dd740c8d9b049c784de17a9981f3c335bc364fcf19d02a` | 5238 | operation |
| [v2/labels/create-label](https://developers.plane.so/api-reference/v2/labels/create-label.md) | `f3e84dbc3864b4a7705a1f1d329dc3fddec319b598c902c5376cd01a534cadd3` | 9464 | operation |
| [v2/labels/update-label](https://developers.plane.so/api-reference/v2/labels/update-label.md) | `1f32e91ab1fd2d9498420f64fd22ad9dc7352c6c26860b1a2805a9e2fdeae8c5` | 9539 | operation |
| [v2/labels/delete-label](https://developers.plane.so/api-reference/v2/labels/delete-label.md) | `baaacb45962d59dec264f317887d5b4c599feade9dd13aaa32304a3596009dc7` | 7539 | operation |
| [v2/labels/upsert-label](https://developers.plane.so/api-reference/v2/labels/upsert-label.md) | `5fbd4e444bfaed90bf7775ee4b4869add885d0854a24cd831302b67e31d08ed6` | 8821 | operation |
| [v2/labels/bulk-create-labels](https://developers.plane.so/api-reference/v2/labels/bulk-create-labels.md) | `105b111b3a76c9a3ddc62b9838de66e1e981374197ad024721163f982a82bbb9` | 10018 | operation |
| [v2/labels/bulk-update-labels](https://developers.plane.so/api-reference/v2/labels/bulk-update-labels.md) | `968c445e8282d55cb1b5a00707fab377ff32bb1a60517074d6533e83c94daf06` | 8606 | operation |
| [v2/labels/bulk-delete-labels](https://developers.plane.so/api-reference/v2/labels/bulk-delete-labels.md) | `3e152526292bffc185c445c6eabc7043aa9ebe69edb5f5d14a902fb5763016cf` | 8102 | operation |
| [v2/project-states/overview](https://developers.plane.so/api-reference/v2/project-states/overview.md) | `8bf77cc7a12e73ce887c51fc885b344260ab7f348ec080198cd0c54b3b1131b9` | 5318 | guide |
| [v2/project-states/list-project-states](https://developers.plane.so/api-reference/v2/project-states/list-project-states.md) | `cbbcf5c6cb513630b36aa349b9585b990f8b41c042faaa4dee7588c1f4471343` | 9077 | operation |
| [v2/project-states/get-project-state](https://developers.plane.so/api-reference/v2/project-states/get-project-state.md) | `de3e249b15bffe00b2607a08cc6c91ca1965914be465bed99f43e3cbb73da93d` | 4596 | operation |
| [v2/project-states/create-project-state](https://developers.plane.so/api-reference/v2/project-states/create-project-state.md) | `22a98f78fff058a46a112fe3d938f1cc83f2eec6bc840a8f74b1c6ee805f1c1f` | 7771 | operation |
| [v2/project-states/update-project-state](https://developers.plane.so/api-reference/v2/project-states/update-project-state.md) | `82aa014acad905d80d8a07296d052d7bd8dc1f280afb4918331dbacdd6f209a1` | 7610 | operation |
| [v2/project-states/delete-project-state](https://developers.plane.so/api-reference/v2/project-states/delete-project-state.md) | `4f0b91a2d6d31af05644ee63a0207c13ebbe0e52aab2b39ca33dda485a56eb3d` | 5917 | operation |
| [v2/project-states/mark-default-project-state](https://developers.plane.so/api-reference/v2/project-states/mark-default-project-state.md) | `c3f66bce41a53a7f65489dc452fcfc8f4de6351df7a1a800c2b21711eb81d762` | 6102 | operation |
| [v2/project-labels/overview](https://developers.plane.so/api-reference/v2/project-labels/overview.md) | `db4dd3ac02e03952f3f6b9e406f31c082482e89f254dd76c08a8433167544477` | 4423 | guide |
| [v2/project-labels/list-project-labels](https://developers.plane.so/api-reference/v2/project-labels/list-project-labels.md) | `9bc6f96b63252bddd7db24b4997903c18de1d8e7dc69ff661eb2b514778e2151` | 6603 | operation |
| [v2/project-labels/get-project-label](https://developers.plane.so/api-reference/v2/project-labels/get-project-label.md) | `71ed6aa9b833d541cbfb21342c9a84f7d43d22bc42ff9be75428a675d5dd227b` | 4074 | operation |
| [v2/project-labels/create-project-label](https://developers.plane.so/api-reference/v2/project-labels/create-project-label.md) | `1dda51fa6a7a6cc6b6ff5624c027518f3a9cc1be26850e61c4a4e9e7a4649497` | 6706 | operation |
| [v2/project-labels/update-project-label](https://developers.plane.so/api-reference/v2/project-labels/update-project-label.md) | `b937ff41fa1e04db3d951acbd83ab233b62dc1b7f74c7d3988809be9a8ddecc6` | 6719 | operation |
| [v2/project-labels/delete-project-label](https://developers.plane.so/api-reference/v2/project-labels/delete-project-label.md) | `94e90f2be81084a6106b2936766dbfc2736a448687e615f36e7b855732e3b232` | 4896 | operation |
| [v1/project-labels/overview](https://developers.plane.so/api-reference/project-labels/overview.md) | `716583ca2768f3b1f41fbba81115accae90f09895bf17f8b64279f7c4fbcfae5` | 1721 | guide |
| [v1/project-labels/add-project-label](https://developers.plane.so/api-reference/project-labels/add-project-label.md) | `8b671194ecf5364f547d461ba6f60a32bb68a54580e17294130cca724bd065e5` | 3221 | operation |
| [v1/project-labels/list-project-labels](https://developers.plane.so/api-reference/project-labels/list-project-labels.md) | `23b62b10cd05be5ecb565a652577a18a7d0ef4544e4cd3197a268cd05bf2e79d` | 3112 | operation |
| [v1/project-labels/get-project-label-detail](https://developers.plane.so/api-reference/project-labels/get-project-label-detail.md) | `7ba476f0cc9d918fde28b71f081ce2f77de1ab3241bca47bc8b52d0a2947efcf` | 2383 | operation |
| [v1/project-labels/update-project-label-detail](https://developers.plane.so/api-reference/project-labels/update-project-label-detail.md) | `52f3150bf129aa73153a83b1159519ccb7e9c09da2d48a3b7521cf610db4bbf8` | 3391 | operation |
| [v1/project-labels/delete-project-label](https://developers.plane.so/api-reference/project-labels/delete-project-label.md) | `b1ea10cc1a6a551464df6e2860d445e5930bea0c81a5a7dd42482f5e9c69258d` | 2029 | operation |
| [v1/state/overview](https://developers.plane.so/api-reference/state/overview.md) | `7bbe302436261f6997eaf6cf3654624cf5ae0732e7570f1e65d6b4603e967597` | 2140 | guide |
| [v1/state/add-state](https://developers.plane.so/api-reference/state/add-state.md) | `2146b20d83b77102264f10700b983edacf8c138ecb773dcea777e44b3d2f6718` | 4082 | operation |
| [v1/state/list-states](https://developers.plane.so/api-reference/state/list-states.md) | `c43eb3fb91d22ac3e64571fb463deeeb0824930d68df9e1720c85512deb92717` | 3140 | operation |
| [v1/state/get-state-detail](https://developers.plane.so/api-reference/state/get-state-detail.md) | `43d948ea93758d33f3085fd45be2f430bdb4b039aa2576b552b4ff215fc76328` | 2359 | operation |
| [v1/state/update-state-detail](https://developers.plane.so/api-reference/state/update-state-detail.md) | `277f62c282b0133e1830312650980bcdc2c66f76f8a760454b415267e3742282` | 4290 | operation |
| [v1/state/delete-state](https://developers.plane.so/api-reference/state/delete-state.md) | `f3412ea03aa95b70ffa5960eb7d05be1d4ca688b317fbce9536b1e950262bb0d` | 2219 | operation |
| [v1/label/overview](https://developers.plane.so/api-reference/label/overview.md) | `d2ef9e86116b02f42fc8be0f306dc3e5318e256ba555d0fba4af7cd249930247` | 1739 | guide |
| [v1/label/add-label](https://developers.plane.so/api-reference/label/add-label.md) | `3bca61cdc27a8ae57d4ad035bdd6c7194b7aee226b6074f03ed34b633efb2bbd` | 3818 | operation |
| [v1/label/list-labels](https://developers.plane.so/api-reference/label/list-labels.md) | `e0c2817f2945bdb74dd50e97a5a9627af7448cd89f513b6ca156234498a46945` | 3306 | operation |
| [v1/label/get-label-detail](https://developers.plane.so/api-reference/label/get-label-detail.md) | `2113b4d7945c8e607436e96ce0e2309c73681c753033bdda9e12b494f78713ec` | 2340 | operation |
| [v1/label/update-label-detail](https://developers.plane.so/api-reference/label/update-label-detail.md) | `9dc0c4c9e502d4f4bd2eacfc19b5d9412bb1df35effb4c04ab7db35ecbf3147a` | 3980 | operation |
| [v1/label/delete-label](https://developers.plane.so/api-reference/label/delete-label.md) | `719a38bb38e2f3cd7d21fb1b8f668044214b40e40031535f46c520719fa3dc24` | 2169 | operation |

## Consumer acceptance authored (not executed)

- `TestStateLabelRequests`: exact method/path/body/output fixtures for every 49 operation pages, including explicit scope, delete 204, workspace governance surfaces, bodyless default marking, writable empty/zero/false/null, native pagination envelope and unknown large integers.
- `TestStateLabelLocalValidation`: omitted scope, workspace-only required fields/group rules, workspace default exclusion, upsert correlation requirements, bulk bounds/UUID identity, destructive guard, duplicate JSON keys, unknown write fields and body-file/field-flag conflict.
- `TestStateLabelNativeBulkPartial`: native row identities and partial output/exit 6.
- `TestStateLabelNativePagination`: v1 continuation and v2 noncomputed next offset, preserved shaping/filter/false flags, two staged pages.
- `TestStateLabelSparseTableAndDryRun`: sparse deterministic table without relation fetches and credential-free deletion dry-run.

No builds, tests, linters, formatters, smoke checks, commits or pushes were run by this worker. Coordinator integration must run focused state/label tests, catalog reconciliation and make check.
