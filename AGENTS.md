# AGENTS.md - Nais API

Guidelines for AI agents working on this project.

## Project structure

- **Go project** with a GraphQL API and gRPC
- Domain-driven design: each package in `internal/` contains one domain
- GraphQL schemas in `internal/graph/schema/*.graphqls`
- SQL queries in `internal/<domain>/queries/*.sql`
- Integration tests in `integration_tests/*.lua`

## Common commands

| Task                      | Command                     |
| ------------------------- | --------------------------- |
| Build                     | `mise run build`            |
| Run locally               | `mise run local`            |
| Run all tests             | `mise run test`             |
| Unit tests only           | `mise run test:unit`        |
| Integration tests with UI | `mise run test:ui`          |
| Generate all code         | `mise run generate`         |
| Generate GraphQL          | `mise run generate:graphql` |
| Generate SQL              | `mise run generate:sql`     |
| Generate mocks            | `mise run generate:mocks`   |
| Format code               | `mise run fmt`              |
| All checks                | `mise run check`            |

## Running specific integration tests

```bash
go test -v -tags=integration_test -run "TestIntegration/<testname>" ./integration_tests
```

## Workflow

1. **Before changing code**: Read the relevant files to understand existing patterns. Whenever modifying `.graphqls` files, first read and follow `.github/instructions/graphql.instructions.md`.
2. **After changing `.graphqls` files**: Run `mise run generate:graphql`
   1. If models end up in `internal/graph/model/donotuse/models_gen.go`, move them to the appropriate package/domain and regenerate
3. **After changing `.sql` files**: Run `mise run generate:sql`
4. **After all changes**: Run `mise run test` and `mise run fmt`
5. **Before delivering PR work**: Run CI's formatting and generation command, `mise run fmt ::: generate`. Review `git status --short` and include all changes to generated files; tests and builds alone do not reveal outdated generated files. Once the changes are committed, `mise run fmt ::: generate && git diff --exit-code` must pass on a clean working tree.
6. **On compilation errors**: Check that generated code is up to date

## Tests

- Use standard Go checks with `t.Error`/`t.Fatal` and their variants, not `testify/assert` or `testify/require`.
- Compare structs using `github.com/google/go-cmp/cmp`, preferably `cmp.Diff` for readable error messages.

## Documentation

- **Development practices**: `docs/practices.md`
- **GraphQL conventions**: `docs/graphql_practices.md`
- **Audit logging**: `docs/audit_events.md`

## Configuration

- GraphQL: `.configs/gqlgen.yaml`
- SQL (sqlc): `.configs/sqlc.yaml`
- Mocks: `.configs/mockery.yaml`

## Local development

```bash
mise install
cp .env.example .env
docker compose up -d
mise run local:setup
mise run local
```

To test as a user, set the header `X-User-Email: dev.usersen@example.com`.
For an admin: `X-User-Email: admin.usersen@example.com`.
