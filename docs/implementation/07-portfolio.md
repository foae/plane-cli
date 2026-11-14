# P07 — Portfolio, customers, releases, templates and views

Prerequisites P03/P05/P06. Read [API design](../design/api.md) and the exact source pages for each bounded family slice.

## Source ownership

V1: `initiative,release,customer,teamspace`. V2: `initiatives,initiative-labels,releases,release-tags,release-labels,release-comments,release-links,project-releases,project-release-comments,project-release-links,customers,customer-properties,customer-property-values,customer-requests,teamspaces,project-views,workspace-views,project-templates,project-work-item-templates,workspace-work-item-templates,project-page-templates,workspace-page-templates,template-categories`.

## Implement

One family slice owns matching CLI/version files, fixtures/catalog entries and documentation. Cover all membership, association, tags/labels, ordering, archive, changelog, apply/copy and other documented actions—not just CRUD. Do not merge workspace releases with project releases or customer requests with intake work items. Keep customer property definitions separate from property values.

Preserve exact release/version identifiers, date formats and template/view payloads; no synthetic semantic-version validation or query-language translation unless the endpoint demands it. Feature entitlement errors remain accurate upstream failures. Views/template filters may have their own JSON contracts; do not feed MCP PQL into them.

## Acceptance

Per-operation fixtures assert correct parent scope and payload. Meaningful edges include workspace/project release isolation, membership removal confirmation, customer property null/empty value updates, template application side effects and feature-gated 402. Required common list/get tables use declared projections. Reconcile catalog and add bounded portfolio/customer/template recipes.
