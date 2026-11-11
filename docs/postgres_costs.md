# Postgres billing costs

## GraphQL contract

`Postgres.cost` exposes imported billing costs independently of consuming
applications. The identity is team + environment + Postgres name; tenant is
implicit in the API deployment. All branches aggregate under their owning
Postgres.

```graphql
fragment PostgresBilling on Postgres {
	cost {
		daily(from: "2026-02-01", to: "2026-02-28") {
			sum
			series {
				date
				sum
				services {
					service
					cost
				}
			}
		}
		monthly {
			sum
			series {
				date
				sum
				services {
					service
					cost
				}
			}
		}
	}
}
```

- Costs are in euros, preserving the existing billing import's currency and
  credit treatment. No resource-price estimates are introduced.
- Daily bounds are inclusive. Equal bounds request a single day. Reversed dates
  and future end dates return GraphQL errors.
- Daily series are oldest first. Only dates with imported attributed samples
  appear; missing dates are not filled with zeroes.
- Monthly covers 12 calendar months ending in the most recent billing month
  imported for the tenant, newest first. Each series date is the last observed
  billing date for this Postgres in that month, across its services.
- `daily` and `monthly` return null when no attributed samples exist. An explicit
  zero-valued sample remains recorded data and can produce a zero sum.
- Query failures return GraphQL errors rather than a successful zero or an empty
  period. Existing legacy Cloud SQL behavior is unchanged.
- Data is delayed by at least two days, plus scheduled billing refresh/import
  time. A non-null period reports observed imported costs; it does not certify
  complete replica, PgBouncer or storage coverage.

Service breakdown uses the existing billing service dimension, not an inferred
Postgres service name. Totals sum all attributed rows once; the billing pipeline
must aggregate replicas, PgBouncer and verified storage before ingestion.

## Storage and import compatibility

Migration `0083_postgres_cost_identity.sql` adds nullable `cost.postgres_name`
without truncating historical costs. The uniqueness key includes this identifier
and keeps existing null-equality behavior. A database constraint rejects empty
Postgres identifiers and rows carrying both a Postgres identity and an app label.

The importer accepts a nullable `postgres_name` column from the tenant BigQuery
view. A missing column leaves the identifier unset, preserving existing imports.
For Postgres rows, the importer clears the app label to its existing null-app
representation (empty string), preserves service dimensions, and requires a
nonempty Postgres name, team and environment. Invalid rows and iterator failures
fail the fetch instead of being silently skipped.

Workload queries explicitly exclude Postgres rows. Team/tenant totals retain them.
Imports remain upserts, not authoritative date-window replacements: changing a
row's attribution does not delete its old identity automatically.

## Rollout gates

This API patch is not sufficient for enabling production attribution:

1. Verify exported label keys and actual database/replica/PgBouncer/storage billing
   coverage. Parent/template metadata alone is not proof of exported coverage.
2. Review and test BigQuery changes, including aligned INSERT/UNION columns,
   grouping, tenant filtering, service dimensions and unchanged credit handling.
3. Deploy the additive API migration and compatible importer before tenant views
   begin delivering Postgres rows. An old importer cannot distinguish separate
   Postgres resources. Confirm other shared-view consumers' compatibility.
4. Agree a controlled replacement/reimport for affected billing dates. If rows
   were previously attributed to apps or unowned services, upserting their new
   Postgres identity alone leaves stale costs and double-counts totals. Do not
   delete costs merely because an app and Postgres share a name; both may be real.
   Rebuild a reviewed, complete date/team scope from authoritative tenant-view
   data, with backup, row-count/total reconciliation and rollback available.
5. Newly introduced labels cannot recover historical attribution. Limit backfill
   to verified labeled history and account for the billing refresh/import windows.
6. Reconcile resource totals against billing rows and unchanged overall totals
   before considering coverage complete. No live BigQuery changes are included
   in this repository patch.

## Validation

Fast tests (no containers):

```bash
go test -race ./internal/cost/... ./internal/graph
```

Database-backed regression (requires Docker and reachable container ports; run
outside a sandbox that disallows local services):

```bash
go test -tags=integration_test ./internal/cost \
  -run '^TestPostgresCostStorageAndQueries$' -count=1
```

The database test exercises migration preservation, identity isolation, repeat
imports/corrections, inclusive dates, explicit zeroes, the 12-month boundary,
application exclusion, and unchanged team/tenant totals. Full repository checks
also require Docker for integration tests and generated tester specifications.
