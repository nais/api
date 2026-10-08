# AGENTS.md - Nais API

Kjøreregler for AI-agenter som jobber med dette prosjektet.

## Prosjektstruktur

- **Go-prosjekt** med GraphQL API og gRPC
- Domene-drevet design: pakker i `internal/` inneholder ett domene hver
- GraphQL schema i `internal/graph/schema/*.graphqls`
- SQL queries i `internal/<domain>/queries/*.sql`
- Integrasjonstester i `integration_tests/*.lua`

## Vanlige kommandoer

| Oppgave                   | Kommando                    |
| ------------------------- | --------------------------- |
| Bygg                      | `mise run build`            |
| Kjør lokalt               | `mise run local`            |
| Kjør alle tester          | `mise run test`             |
| Kun enhetstester          | `mise run test:unit`        |
| Integrasjonstester med UI | `mise run test:ui`          |
| Generer all kode          | `mise run generate`         |
| Generer GraphQL           | `mise run generate:graphql` |
| Generer SQL               | `mise run generate:sql`     |
| Generer mocks             | `mise run generate:mocks`   |
| Formater kode             | `mise run fmt`              |
| Alle sjekker              | `mise run check`            |

## Kjøre spesifikke integrasjonstester

```bash
go test -v -tags=integration_test -run "TestIntegration/<testnavn>" ./integration_tests
```

## Arbeidsflyt

1. **Før du endrer kode**: Les relevante filer for å forstå eksisterende mønstre
2. **Etter endringer i `.graphqls`**: Kjør `mise run generate:graphql`
   1. Hvis det havner modeller i `internal/graph/model/donotuse/models_gen.go`, flytt disse til riktig pakke/domene og generer på nytt
3. **Etter endringer i `.sql`**: Kjør `mise run generate:sql`
4. **Etter alle endringer**: Kjør `mise run test` og `mise run fmt`
5. **Før PR-arbeid leveres**: Kjør CI sin format- og genereringskommando `mise run fmt ::: generate`. Se gjennom `git status --short` og ta med alle endringer i genererte filer; tester og bygg alene avslører ikke utdaterte genererte filer. Når endringene er committet, skal `mise run fmt ::: generate && git diff --exit-code` passere på ren arbeidskopi.
6. **Ved kompileringsfeil**: Sjekk at generert kode er oppdatert

## Dokumentasjon

- **Utviklingspraksis**: `docs/practices.md`
- **GraphQL-konvensjoner**: `docs/graphql_practices.md`
- **Audit logging**: `docs/audit_events.md`

## Konfigurasjoner

- GraphQL: `.configs/gqlgen.yaml`
- SQL (sqlc): `.configs/sqlc.yaml`
- Mocks: `.configs/mockery.yaml`

## Lokal utvikling

```bash
mise install
cp .env.example .env
docker compose up -d
mise run local:setup
mise run local
```

For å teste som bruker, sett header `X-User-Email: dev.usersen@example.com`.
For admin: `X-User-Email: admin.usersen@example.com`.
