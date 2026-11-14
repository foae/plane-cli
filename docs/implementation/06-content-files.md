# P06 — Pages, collections, assets and attachments

Prerequisite P03. Read [API files](../design/api.md#files-pages-and-secrets), [transport isolation](../design/authentication.md#transport-isolation), [CLI files](../design/cli-contract.md#tables-and-files).

## Source ownership

V1: `page,collection,sticky,assets,issue-attachments,work-item-pages`. V2: `project-pages,workspace-pages,workspace-page-attachments,collections,published-pages,stickies,user-assets,workspace-assets,work-item-attachments,work-item-pages`. Resolve the missing project-page attachment overview through documented actual asset routes; never invent it.

## Implement

Add page/collection/sticky/link commands and their complete documented actions. Preserve HTML and opaque editor metadata; do not invent Markdown conversion or normalize arbitrary JSON. Handle page tree/archive/unpublish/delete dependencies explicitly and classify cascading operations for --yes.

Implement documented attachment/asset phase operations **and** upload/download convenience workflows. Work-item v2 metadata POST expects 200 and signed multipart fields, followed by separate storage transfer and confirmation. Inspect each other family/version separately; public schema, server upload and presigned storage are not interchangeable. Shared transfer helpers own secure local IO and credential isolation, not endpoint assumptions.

Require protected explicit output for raw signed material. Stream files, sanitize filenames, verify supplied size/checksum, reject unsafe redirects, and atomically install downloads without accidental overwrite. Report failed confirmation or other partial writes with safe phase/asset IDs; no automatic rollback or retry.

## Acceptance

Three-origin fixture distinguishes API/OAuth policy from credential-free object storage. Assert exact multipart fields/file bytes and that API auth never reaches storage. Test failed phase 1/2/3, cancel, oversized/missing local file, signed URL redaction, HTTP-storage opt-in, unsafe filename, existing output destination and explicit overwrite. Page fixtures cover nested archive/unpublish/delete requirements. Update content/file recipes and safe recovery instructions.
