# P06 — Pages, collections, assets and attachments

Prerequisite P03. Read [API files](../design/api.md#files-pages-and-secrets), [transport isolation](../design/authentication.md#transport-isolation), [CLI files](../design/cli-contract.md#tables-and-files).

## Source ownership

V1: `page,collection,sticky,assets,issue-attachments,work-item-pages`. V2: `project-pages,workspace-pages,workspace-page-attachments,collections,published-pages,stickies,user-assets,workspace-assets,work-item-attachments,work-item-pages`. Resolve the missing project-page attachment overview through documented actual asset routes; never invent it.

P06 also owns evidence and implementation for supplemental public v1 `/server/` uploads and other server-mediated variants. Resolve these before this packet closes; P09 only reconciles the ledger, not a late prerequisite research phase.

## Implement

Add page/collection/sticky/link commands and their complete documented actions. Preserve HTML and opaque editor metadata; do not invent Markdown conversion or normalize arbitrary JSON. Handle page tree/archive/unpublish/delete dependencies explicitly and classify cascading operations for --yes.

Implement documented attachment/asset phase operations **and** upload/download convenience workflows. Work-item v2 metadata POST expects 200 and signed multipart fields, followed by separate storage transfer and confirmation. Inspect each other family/version separately; public schema, server upload and presigned storage are not interchangeable. Shared transfer helpers own secure local IO and credential isolation, not endpoint assumptions.

Use the shared intentional-secret output contract: phase commands expose signed material in JSON by default with opt-in protected-file delivery; convenience transfers do not expose it. Stream files, sanitize filenames, verify supplied size/checksum, follow only documented redirect phases and atomically install downloads without accidental overwrite. Trust documented HTTP/HTTPS storage targets, including private hosts and HTTP storage returned by HTTPS APIs, without allowlists, opt-ins or warnings. Report failed confirmation or delivery with safe phase/asset IDs; no automatic rollback or retry.

## Acceptance

Three-origin fixtures distinguish API/OAuth from credential-free storage. Assert exact multipart fields/file bytes and no Plane auth at storage. Test failed phases, cancel, documented size-limit violations/missing files, intentional signed output versus diagnostic redaction, HTTP storage with an HTTPS API without flags/warnings, unsafe filenames, existing destination and explicit overwrite. Test protected-output preflight failure sends no mutation, and post-mutation delivery failure is partial without retry or stdout fallback. Page fixtures cover nested archive/unpublish/delete requirements. Update recipes and safe recovery instructions.
