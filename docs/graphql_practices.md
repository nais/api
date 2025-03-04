# GraphQL practices

The following is a set of practices for developing the GraphQL API defined by Nais API.

## Type conventions

### The `id` field

Nais API implements the [Global Object Identification specification](https://graphql.org/learn/global-object-identification/).

All types should have an `id` field of type `ID!`. We create IDs using the internal `ident` package. Refer to the
implementation in the internal [`teams` package](../internal/team/node.go) for an example.

Types that implement the `Node` interface can be fetched using the `node` query:

```graphql
query getNodeById {
	node(id: "opaque id") {
		... on Team {
			slug
		}

		... on User {
			email
		}

		... on Application {
			name
			team {
				slug
				purpose
			}
		}
	}
}
```

### Naming

Types should be named as follows: `Noun`.

Examples: `Team`, `User`, `Repository`.

### Fields

Field should be camelCased.

Examples: `id`, `createdAt`.

## Documentation

All types, fields, queries, and mutations should be documented using
[Descriptions](https://spec.graphql.org/October2021/#sec-Descriptions) in the schema definition. Please refer to the
[`teams` schema](../internal/graph/schema/teams.graphqls) for examples. Keep in mind that this will be public
facing documentation, and the entry point for developer who wants to use the GraphQL API.

## Query conventions

We want to reduce the number of queries and mutations to a minimum, and utilize the graphing capabilities of GraphQL.

Instead of having multiple queries returning a list of items, we have a single query with filters and pagination.

Example:

```graphql
# instead of having these queries:
type Query {
	itemsByX(x: String!): [Item!]!
	itemsByY(y: String!): [Item!]!
	itemsByZ(z: String!): [Item!]!
}

# do this:
type Query {
	items(filter: ItemsFilter): ItemsConnection!
}

input ItemsFilter {
	x: String
	y: String
	z: String
}
```

We also want to avoid using queries on the root level that has a single argument which is an ID of another type.

Example:

```graphql
# instead of having this query:
type Query {
	teamUtilization(teamSlug: Slug!): Utilization!
}

# do this:
type Team {
	utilization: Utilization!
}
```

### Pagination

Nais API implements the [GraphQL Cursor Connections specification](https://relay.dev/graphql/connections.htm).

All queries/fields that return a list of items should support pagination. The rule of thumb is that if it is
**possible** for the list to grow to a size that is more than 50, it should be paginated.

The type of the returned list from the query is `TypeNameConnection` (`Connection` suffix) and should be defined as
follows:

```graphql
type TypeNameConnection {
	pageInfo: PageInfo!
	nodes: [TypeName!]!
	edges: [TypeNameEdge!]!
}

type TypeNameEdge {
	cursor: Cursor!
	node: TypeName!
}
```

The `nodes` field is not a part of the spec, and is included for convenience.
`nodes` and `edges` are both of the same length, and contain the nodes in the same order.
I.e. `nodes[n]` is the same as `edges[n].node`.

`PageInfo` is defined as follows:

```graphql
type PageInfo {
	hasNextPage: Boolean!
	endCursor: Cursor
	hasPreviousPage: Boolean!
	startCursor: Cursor
	totalCount: Int!
}
```

Refer to the [`teams` query](../internal/graph/schema/teams.graphqls) in the schema for an example.

### Filtering

Instead of using multiple queries or arguments to filter a list, we use a single `filter` argument. The `filter`
argument is a type that contains all the possible filters for the list. The filter should be named `TypeNameFilter`
(`Filter` suffix) and should be defined as follows:

```graphql
type Query {
	typeName(filter: TypeNameFilter): TypeNameConnection!
}

input TypeNameFilter {
	fieldX: String
	fieldY: Int
	nestedField: TypeNameNestedFieldFilter
}

input TypeNameNestedFieldFilter {
	fieldX: String!
	fieldY: Int!
}
```

## Mutation conventions

### Naming

Mutations should be named as follows: `actionNoun` (`action` prefix, `Noun` suffix).

Examples: `createTeam`, `deleteTeam`, `updateTeam`.

### Input

Use a single, required, unique, input object type as an argument for mutations instead of multiple arguments. The name
of the argument should be `input`.

Example:

```graphql
# instead of this:
type Mutation {
	createTeam(name: String!, description: String!): CreateTeamPayload!
}

# do this:
type Mutation {
	createTeam(input: CreateTeamInput!): CreateTeamPayload!
}

input CreateTeamInput {
	name: String!
	description: String!
}
```

### Response payloads

Each mutation should have a unique response object type. The name of the response object should be `ActionNounPayload`.

Example:

```graphql
type Mutation {
	createTeam(input: CreateTeamInput!): CreateTeamPayload
}

input CreateTeamInput {
	name: String!
	description: String!
}

type CreateTeamPayload {
	team: Team
}
```

## Deprecating fields / queries / mutations

When a field, query, or mutation is deprecated it should be marked as such in the schema definition. The
[`@deprecated` directive](https://spec.graphql.org/October2021/#sec--deprecated) should be used with a reason for the
deprecation.
