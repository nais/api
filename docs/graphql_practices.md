# GraphQL

This is a set of practices for the Graph-API.

## Type conventions

### Naming

Types should be named as follows: `Noun`.
For example: `Team`, `User`, `Repository`.

### Fields

Field should be camelCased.

### ID

All types should have an `id` field of type `ID!`.
This should be a unique identifier for the type, and is not necessarily the same as the ID in the underlying system.

## Query conventions

We want to reduce the number of queries and mutations to a minimum, and utilize the graphing capabilities of GraphQL.

E.g. instead of having multiple queries returning a list of items, we have a single query with filters and pagination.

Avoid using queries on the root level that has a single argument which is and ID of another type.

### Pagination

In general, all queries/fields that return a list of items must support pagination.
The pagination arguments are: `offset` and `limit`.
`offset` is the number of items to skip, and `limit` is the maximum number of items to return.

The rule of thumb is that if it is **possible** for the list to grow to a size that is more than 50, it should be paginated.

The type of the returned list is `TypeNameList` (`List` suffix) and should be defined as follows:

```graphql
type TypeNameList {
  nodes: [TypeName!]!
  pageInfo: PageInfo!
}
```

Where `TypeName` is the type of the items in the list, and `PageInfo` is defined as follows:

```graphql
type PageInfo {
  totalCount: Int!
  hasNextPage: Boolean!
  hasPreviousPage: Boolean!
}
```

### Filtering

Instead of using multiple queries or arguments to filter a list, we use a single `filter` argument.
The `filter` argument is a type that contains all the possible filters for the list.
The filter should be named `FieldNameFilter` (`Filter` suffix) and should be defined as follows:

```graphql
extend type Query {
  teams(offset: Int, limit: Int, filter: TeamsFilter): TeamsList! @auth
}

input TeamsFilter {
  github: TeamsFilterGitHub
}

input TeamsFilterGitHub {
  "Filter repostiories by repo name"
  repoName: String!
  "Filter repostiories by permission name"
  permissionName: String!
}
```

In this example we have a query that returns a list of teams.
The filter contains a field `github` which is an optional filter, but has two required fields: `repoName` and `permissionName`.

## Mutation conventions

### Naming

Mutations should be named as follows: `actionNoun` (`action` prefix, `Noun` suffix).
For example: `createTeam`, `deleteTeam`, `updateTeam`.

## Federation

We use Federation 2 to compose the supergraph.
This means that each service is responsible for its own schema, and the supergraph is composed by stitching together the schemas.

Each service must have a `@key` directive on the type that the service is responsible for.

```graphql
type Team @key(fields: "id") {
  id: ID!
  name: String!
}
```

The `@key` directive specifies which fields are used to uniquely identify the type.
