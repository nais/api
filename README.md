# Nais API

Nais API is the main entrypoint when interacting with the API of the Nais platform.

## Local development

```bash
mise install # Install required dependencies
cp .env.example .env # Copy example configuration file, and adjust to your needs
docker compose up -d
make setup-local # Setup local environment (seeds database and creates pubsub topic)
make local
```

For local development various services are faked.
This is controlled by multiple `WITH_*` environment variables in the `.env` file.
If `WITH_INSECURE_USER_HEADER` is set, you will also need to set the `X-User-Email` header to `dev.usersen@example.com` if you want
to act as a regular user, or `admin.usersen@example.com` if you need an admin user, when interacting with the GraphQL API.

Locally, most external services are replaced with fake clients, and the Kubernetes client reads data from `data/k8s`.

The test suite uses [testcontainers](https://testcontainers.com/), and if you are using Colima you might need to follow
the guidelines in the [testcontainers documentation](https://golang.testcontainers.org/system_requirements/using_colima/).

### Local development of logging API

To develop the logging API with real data:

```bash
TENANT_NAME="example.com"
nais login
narc jita grant k8s-admin "$TENANT_NAME" --duration 1h --reason "port forward for loki local testing"
CONTEXT="dev" WITH_LOG_PROXY="true" mise run local|debug
```

## Kafka environments

`KAFKA_ENVIRONMENTS` maps canonical Nais environment names to Kafka
pools and optional legacy aliases. Temporary credentials are created in the
selected canonical environment, using the existing cluster-name mapping.
For example, for Nav:

```json
{
	"dev-gcp": {
		"pool": "nav-dev",
		"aliases": ["dev"]
	},
	"prod-gcp": {
		"pool": "nav-prod",
		"aliases": ["prod"]
	}
}
```

The Helm value is the JSON string `kafka.environments`. For ordinary
tenant environments, Fasit generates a mapping when both `aiven_project` and
`kafka_bootstrap_broker` are configured. The pool is the configured Aiven project,
and credentials are created in the same Nais environment, without aliases.
Nav retains the explicit legacy mappings above.

Non-standard topologies can configure an explicit mapping. A direct Helm
installation must also supply this value. An empty mapping disables Kafka
credential creation, not other Kafka operations.

The canonical environment must refer to a configured Kubernetes cluster through the existing environment-name
mapping. Aliases cannot collide with canonical Nais environments or other aliases.
Pool names are never inferred from environment names or the tenant name.
An environment without a mapping is rejected before creating any resources.
Selecting a shared pool does not route credential creation to another cluster.
There is no routing from FSS to GCP.

Existing clients can keep using configured aliases. The additive
`CreateKafkaCredentialsPayload.environmentName` field returns the canonical name,
allowing newer clients to warn about legacy aliases without duplicating this
configuration. Activity log entries use the canonical name.

## v0s migration

See the [Console migration contract](docs/v0s_console_migration.md) for existing
support, backend prerequisites, counting semantics and the proposed rollout.
Proposed additions in that document are not yet implemented.

The new vulnerability fields read v0s through the public
[v0s gRPC client module](https://github.com/nais/v0s/tree/main/api). Existing
v13s queries and mutations keep their existing source and behavior.

Set `V0S_ENDPOINT` to the plaintext gRPC endpoint allowed by v0s's NetworkPolicy.
The Helm chart sets it from `v0s.endpoint`, defaulting to `v0s:50051`, and allows
egress to pods labelled `app: v0s` in the same namespace on TCP port 50051.
Override `v0s.endpoint` with an empty string to disable the new reads for a tenant.
This does not change the separate v13s endpoint or enable assessment writes.
The connection is lazy: an unset endpoint or unavailable v0s does not block API
startup, workload inventory, or legacy vulnerability fields. New fields return
errors rather than silently falling back to v13s or reporting zero findings.
Environment names are the same user-facing names used by `Workloads.ListImages`.

`V0S_SUPPRESSIONS_ENABLED` defaults to false. Keep it false for the Console
migration until durable activity-log delivery is implemented and verified.
Currently the assessment batch commits in v0s before activity-log entries are
written separately in API, so a log error can follow a successful assessment
write. The new mutations authorize the whole team's batch and derive the actor
from the authenticated user. Deletions do not require an active finding.
Existing v13s mutations remain available for legacy clients. Importing old
assessments, and stopping any external importer before it can overwrite v0s
assessments, are separate rollout responsibilities; no assessment sync is
established by configuring this API.

New lists are forward-only (`first` from 1 to 200) and use the standard
`edges`, `nodes` and `PageInfo` contract. `totalCount` covers the full filtered
selection and is loaded only when requested; counting suppressions currently
pages through the matching assessments. `pageStart` and `pageEnd` track positions
in the traversal; concurrent updates can change the total. Findings and
suppressions are available at the root and scoped through `Team` and
`TeamEnvironment`. Findings are also available on each workload.
`vulnerabilityFindings.facets` describes the full filtered selection, including
when filtering by multiple vulnerability identifiers or aliases.
Use `filter: {packageType: "npm"}` to select an ecosystem (case-insensitive).
`facets.packageTypes { value count }` returns finding counts for the entire
filtered selection, including the package-type filter, not only the current page.
Removed workloads and obsolete deployed images are excluded using an explicit
backend workload selection.
Findings, advisories and assessments have global IDs and can be fetched through
`node`. A finding's ID includes its image digest, so an obsolete-image finding
is not returned after the workload changes images. Assessments for workloads
missing from current inventory are not listed or returned by `node`, but are
not deleted from v0s. The activity log stores assessment details independently.

Finding `orderBy` accepts `SEVERITY` or `IDENTIFIER` with `ASC` or `DESC`.
Severity DESC is CRITICAL, HIGH, MEDIUM, LOW, UNKNOWN; ASC reverses the known
severities, with UNKNOWN still last. Severity ties use identifier ASC, then
environment/team/workload/package URL ASC. Identifier ordering is bytewise and
case-sensitive, with environment/team/workload/package URL ASC for ties.
Omitting `orderBy` preserves environment/team/workload/identifier/package URL ASC.
For example:

```graphql
{
	team(slug: "my-team") {
		vulnerabilityFindings(
			first: 20
			filter: { packageType: "npm" }
			orderBy: { field: SEVERITY, direction: DESC }
		) {
			nodes {
				vulnerabilityIdentifier
				severity
				purl
			}
			facets {
				packageTypes {
					value
					count
				}
			}
			pageInfo {
				hasNextPage
				endCursor
				totalCount
			}
		}
	}
}
```

Keyset cursors wrap v0s's per-item tokens. Finding cursors are bound to the
operation, normalized filter, ordering and current workload selection;
assessment cursors are bound to operation and filter.
They remain usable when an earlier finding disappears, and cannot be used
with the old offset-paginated fields. Deploy v0s with per-item cursors and
the pinned client contract's ordering, package facets and workload selectors
before configuring `V0S_ENDPOINT` for this API. Existing finding cursors from
before this integration must be discarded. Restart without a cursor after
changing filters, ordering or workload selection. Pages are live, not a snapshot
across calls; changes to sort values can move findings between pages.

`workloads.facets.vulnerabilityCoverage` counts scan statuses across the full
workload selection. Use `Team.workloads(filter: {vulnerabilityCoverageStatuses:
[PENDING]})` to find workloads awaiting a scan. Coverage uses current API
inventory: workloads missing from v0s or associated with an old image are
pending, not clean. Removed workloads and old-image findings are excluded.

The v0s protobuf client is imported from `github.com/nais/v0s/api`, pinned in
`go.mod`. Updating the client requires a published upstream contract and a
compatible deployed backend, not changes to this repository's proto generation.
Finding identifier filters use `vulnerability_ids` even for a single ID;
the former singular filter field is reserved in the current contract.
Unique vulnerability groups are available through
`vulnerabilityFindings.vulnerabilities(first:, after:, orderBy:)`. They inherit
the full finding filter and scope, not the parent finding page or ordering.
The default group order is severity DESC, UNKNOWN last, then identifier ASC.
Groups also support identifier and affected-workload-count ordering in either
direction. `pageInfo.totalCount` counts unique vulnerabilities, including on an
empty continuation page; each row distinguishes `findingCount` from
`affectedWorkloadCount`. Severity is the highest known severity among matching
findings, or UNKNOWN if all matching findings are UNKNOWN.

Keep using existing API workload lists and their filters, sorting and pagination.
Add `nodes { vulnerabilityOverview { ... } }` for counts fetched in request-scoped
batches from v0s.
The batch loader uses the simplified `v0.11.0` contract and follows
`next_page_token`; it does not depend on workload row cursors or custom ordering.
Each overview contains finding and unique-vulnerability counts,
unique KEV counts, unique-vulnerability severity buckets, and unfiltered coverage.
These derived rows have no independent node identity; navigate to the existing
workload or advisory instead. Missing v0s workloads and obsolete image references
remain in the API list with PENDING coverage and zero known findings, not a clean
result. Backend failures return errors, not pending zero-count overviews.
The overview filter changes counts, not workload-list membership or coverage.
`Team.vulnerabilityFindings(workloadFilter:)` shares environment/label selection
with `Team.workloads(filter:)`, including all matching workloads in coverage.
Labels retain AND matching, key existence for omitted values, and exact matching
for explicit empty strings. The new v0s paths reject other workload predicates.
For v0s sorting, use `VULNERABILITY_UNIQUE_CRITICAL`, `VULNERABILITY_UNIQUE_HIGH`,
`VULNERABILITY_UNIQUE_MEDIUM`, `VULNERABILITY_UNIQUE_LOW` or
`VULNERABILITY_UNIQUE_UNKNOWN`. Pass the same `filter.vulnerabilityFilter` to the list
and `filter` to each row overview. Sorting uses these unique-severity counts,
not finding counts or v13s statistics.
On `Team.vulnerabilityFindings`, keep finding predicates in its existing `filter`
argument; `workloadFilter` accepts only environment/label selection.
Incomplete scans are first in both directions; scanned workloads then sort by
count, with name/environment/type ascending as stable ties. API loads selected
team aggregates in batches before paging, without fetching all findings.
New-mode cursors bind normalized selection and ordering, keep existing
forward/backward workload pagination, and are incompatible with legacy cursors.
Pages are live offsets, not snapshots. See the
[shared-filter query example](docs/v0s_console_migration.md#additive-graphql-navigation).
No v0s change or new client release is required.
Global affected-only workload selection is not integrated; legacy vulnerability
sorting still uses v13s.
Tenant and team summaries are available as `vulnerabilityFindings.summary`,
including through `Team` and `TeamEnvironment`. They use the full scope and
finding filter, independently of page size, cursor and ordering. The response
distinguishes findings, unique vulnerabilities, affected workloads, unique KEV
vulnerabilities and KEV-affected workloads. Severity buckets count unique
vulnerabilities at their highest known matching severity across the entire
selection, not the sum of per-workload counts.

Summary coverage always counts all API workloads in scope, independently of
finding filters. Missing or outdated v0s workload inventory contributes PENDING
coverage and no obsolete findings. v0s computes the distinct finding aggregates;
API only reconciles the coverage denominator with its inventory. If v0s's
workload total changes between inventory lookup and aggregation, the request
returns an error and should be retried rather than reporting inconsistent counts.
Inventory reconciliation and backend aggregation are not a cross-service snapshot.

```graphql
{
	vulnerabilityFindings {
		summary {
			uniqueVulnerabilityCount
			affectedWorkloadCount
			coverage {
				totalWorkloads
				scanned
				pending
			}
		}
	}
	team(slug: "my-team") {
		vulnerabilityFindings(filter: { packageType: "npm" }) {
			summary {
				findingCount
				uniqueVulnerabilityCount
				affectedWorkloadCount
				knownExploitedVulnerabilityCount
				knownExploitedWorkloadCount
				countsBySeverity {
					critical
					high
					medium
					low
					unknown
				}
				coverage {
					totalWorkloads
					scanned
					pending
					noSbom
					failed
					unresolvedImage
				}
			}
		}
	}
}
```

## Development practices

Check practices used in this project here: [docs/practices.md](docs/practices.md)

### Adding a product-specific relay access mutation

Follow the [shared relay integration requirements](https://github.com/nais/relay/blob/main/SERVICE-INTEGRATION.md) and fill in the [integration template](https://github.com/nais/relay/blob/main/SERVICE-INTEGRATION-TEMPLATE.md) in the product's design or rollout documentation. A new access must be requested through an authenticated, product-specific NAIS API mutation; document its authorization, target selection and owner-only delivery before enabling it.

### Graph-API

We have defined a set of practices for the Graph-API in the [docs/graphql_practices.md](docs/graphql_practices.md) file.

## Fake clients

We have a set of fake clients, and stubs, that can be used for local development.
One of the clients is a fake Kubernetes client that read data from `data/k8s`.
You can read more about how the fake clients works in the [data/k8s/README.md](data/k8s/README.md) file.

## Integration tests

We are using [tester](https://github.com/nais/tester) for integration tests.
These tests are written in Lua and can be found in the `integration_tests` directory.

A spec file is generated to support auto-completion using the Lua language server.

When running `make test` the integration tests will be run as part of the test suite.
If you only want to run the integration tests, and get information about coverage, you can run `make integration-test`.

To run the integration tests in watch mode, run `make integration_test_ui`.
This will start a web server on `localhost:9876` where you can see the test results.
They will be re-run every time you save a `.lua` file.

## Architecture

```mermaid

graph TD
  API --> Postgres[(psql)]
  Service[Nais Service] --> |GRPC|API
  Clients[External Clients] --> |GraphQL|API
  API -.-> PubSub{PubSub}

  subgraph ext [External data]
    API --> K8s([Kubernetes])
    API --> Prom([Prometheus])
    API --> GA([Google Admin])
    API --> BQ([BigQuery])
  end
```

| Component        | Description                                                                                |
| ---------------- | ------------------------------------------------------------------------------------------ |
| API              | This service                                                                               |
| Postgres         | The postgres database storing state from this application                                  |
| Nais Service     | Services by Nais that requires special communication with API. E.g. Reconcilers and Deploy |
| External Clients | External clients that communicates with the API. E.g. Browsers and NADA                    |
| Kubernetes       | The Kubernetes clusters of the tenant                                                      |
| PubSub           | The PubSub topic used for communication between the API and the Nais Service               |
| Prometheus       | We collect historic data from the tenants prometheus                                       |
| Google Admin     | We use the Google Admin API to manage a list of users                                      |
| BigQuery         | We collect historic cost data from BigQuery                                                |

## Code generated by GitHub Copilot

This repository uses GitHub Copilot to generate code.
