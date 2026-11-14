# P05 — Types, custom properties and schema-driven input

Prerequisite P03. Read [API design](../design/api.md#fields-filters-and-types), [CLI contract](../design/cli-contract.md), official type-mode/governance guides.

## Source ownership

V1: `issue-types,epics`. V2: `work-item-types,work-item-properties,work-item-property-options,work-item-property-contexts,work-item-type-properties,workspace-work-item-types,workspace-work-item-properties,workspace-work-item-property-options,workspace-work-item-type-properties`. Coordinate type-governance/workflows with P09; this packet implements the required schema/mode reads, not workflow mutations.

## Implement

Separate project/workspace scope explicitly. Implement all documented type/property/option/context CRUD, linking/order/schema operations and exact v1 epic operations. Distinguish a property's definition, allowed options, per-type assignment/context constraints and actual custom-field values on a work item.

Support documented property types with their true JSON representation, nullability/cardinality and create/update restrictions. Dynamic server-defined property keys must not be rejected by a stale hardcoded closed schema. Expose schema reads explicitly; ordinary mutation does not silently fetch a schema, change governance mode or retry on another surface.

Keep epic semantics accurate: a matching display name is not proof of a read-only system epic flag. Use the exact documented mechanism, and surface unsupported server behavior rather than creating a normal type and calling it an epic.

## Acceptance

Fixtures cover project versus workspace routes, 409 mode conflicts, readonly field rejection, required/multiple/nullable custom fields, option references, context overrides and native schema responses. CLI discovery shows version/scope differences. Type/property reads inform a separate work-item mutation without hidden extra requests. Update the custom-field/type recipe with a complete working example.
