# Development practices

> [!NOTE]
> The use of `v1` in paths and make targets are only meant to be used during the transition from the old to the new API.
> After the transition is complete, all references to `v1` will be removed.

## Domain driven design

We strive to create packages that contain a single domain, and not packages based on what they are (e.g. `repository`, `service`, `controller`).

The main domains and packages are designed for use by the GraphQL API.
Anything used by the gRPC API will be separated into a different package.

E.g. a package for a `Team` domain should contain all the necessary code to work with `Team` entities.

Use singular names for packages, e.g. `team`, `user`, `application`.

### Creating a package

1. Create a new directory with the domain name in the `internal/v1` directory.
   - We have some folders to group similar domains, like `internal/v1/persistence`.
     Use them if it makes sense.
2. If the domain requires a database, update `.configs/sqlc-v1.yaml` to include the new domain.
   E.g:
   ```yaml
   sql:
     # ...
     - <<: *default_domain
       name: "{{PACKAGE_NAME}} SQL"
       queries: "../internal/v1/{{PACKAGE_NAME}}/queries"
       gen:
         go:
           <<: *default_go
           package: "{{PACKAGE_NAME}}"
           out: "../internal/v1/{{PACKAGE_NAME}}/{{PACKAGE_NAME}}sql"
   ```
3. When you've created your model(s) in the new package, add a line to `.configs/gqlgen-v1.yaml` to automatically bind the model to the GraphQL schema.
   E.g:
   ```yaml
   autobind:
     # ...
     - "../internal/v1/{{PACKAGE_NAME}}"
   ```
4. After you've created your dataloader, add it to `internal/cmd/api/http.go`.

## SQL (Postgres)

We use [sqlc](https://sqlc.dev) to generate Go code from SQL queries.
Each domain should have a `queries` directory with the SQL queries for that domain.
See step 2 in the previous section for how to configure sqlc.

When a `.sql` file is changed, run `make generate-sql-v1` to generate the necessary code.

## Dataloaders

We use`github.com/vikstrous/dataloadgen` to create dataloaders.
Dataloaders are used to batch and cache requests to the database, and are scoped to the request.

Whenever you request a single resource, and there's a way to request multiple resources at once, use a dataloader.

For an example, check the [`internal/v1/team/dataloader.go`](../internal/v1/team/dataloader.go) file.

## GraphQL practices

We have defined a set of practices for the Graph-API in the [graphql_practices.md](graphql_practices.md) file.

Whenever a `.graphqls` file is changed, run `make generate-graphql-v1` to generate the necessary code and documentation.

## Pull request workflow

All code changes must be submitted as pull requests.
The main branch is protected.

### Code review

Code review is suggested for all pull requests, but required for those described below.
If at least one participant of the pull request is familiar with the codebase, the review can be skipped.

All checks should pass before merging.

### Required code review

- Changes that affects the public API (e.g. changes to any `.graphqls` file)
- Changes that introduces or changes practices already described in this document
