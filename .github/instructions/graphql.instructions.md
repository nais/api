---
applyTo: "**/*.graphqls"
---

# GraphQL Code Review Instructions

You are reviewing GraphQL schema files for **Nais API**, a platform API built on domain-driven design. These schemas are public-facing — they are the primary interface for developers using the platform.

## Language & Terminology

- **Never** expose Kubernetes-specific terminology (e.g. Pod, Deployment, StatefulSet, CronJob, ConfigMap, Ingress controller, Service, Namespace). Use platform-appropriate abstractions instead (e.g. "instance", "application", "job", "environment").
- Field descriptions must be written for a developer audience that does not know Kubernetes.
- Avoid internal jargon. Use clear, user-friendly wording.

## Documentation

- **Every** type, field, query, mutation, and enum value **must** have a description using GraphQL description syntax (`"..."` or `""" """`).
- Descriptions are public-facing documentation. They should be clear, concise, and helpful.

## Type Conventions

- All types must have an `id` field of type `ID!` and implement the `Node` interface (Global Object Identification spec).
- Type names: `Noun` (e.g. `Team`, `Application`). No prefixes or suffixes like `Gql` or `Type`.
- Field names: `camelCase` (e.g. `createdAt`, `deploymentState`).

## Query Conventions

- Minimize root-level queries. Prefer navigating the graph from existing types.
- **Do not** add root queries that take an ID of another type as the sole argument. Instead, add the field to that type. Example: use `Team.utilization` instead of `Query.teamUtilization(teamSlug: Slug!)`.
- Consolidate queries: use a single query with a `filter` argument instead of multiple queries for different filters.

## Pagination (Cursor Connections spec)

- Any list field that could grow beyond ~50 items **must** be paginated.
- Paginated fields must return a `<TypeName>Connection` type with:
  - `pageInfo: PageInfo!`
  - `nodes: [<TypeName>!]!`
  - `edges: [<TypeName>Edge!]!`
- Each `<TypeName>Edge` must have `cursor: Cursor!` and `node: <TypeName>!`.
- Paginated fields must accept `first: Int` and `after: Cursor` arguments.

## Filtering

- Use a single `filter` input argument (named `<TypeName>Filter`) instead of multiple query arguments.
- Filter input fields should be optional (nullable) so callers can combine filters freely.

## Mutation Conventions

- **Naming**: `actionNoun` format (e.g. `createTeam`, `deleteApplication`, `updateTeam`).
- **Input**: Always use a single required `input` argument with a unique input type named `<ActionNoun>Input` (e.g. `CreateTeamInput`).
- **Do not** use multiple arguments on mutations.
- **Response**: Each mutation must return a unique payload type named `<ActionNoun>Payload` (e.g. `CreateTeamPayload`).

## Deprecation

- Deprecated fields/queries/mutations must use the `@deprecated(reason: "...")` directive with a clear migration path in the reason.

## Common Review Checks

1. Missing descriptions on any type, field, query, mutation, or enum value.
2. Kubernetes terminology leaking into schema or descriptions.
3. Lists that should be paginated but are not.
4. Root queries that should be fields on an existing type.
5. Mutations not following the `input`/`Payload` pattern.
6. Filter arguments not using a dedicated filter input type.
7. Missing `id: ID!` field or `Node` interface implementation.
