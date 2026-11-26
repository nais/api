# Console migration to v0s

## Status

This document describes the current migration contract and remaining work.
Only fields described as implemented are available; proposed additions must not
be used to advertise readiness. An updated protobuf client alone does not
establish that the running backend implements the requested behavior.

v13s and v0s coexist in Nais API. Existing `cve`, `cves`,
`vulnerabilitySummary`, `vulnerabilitySummaries`, image/history fields,
assessment mutations, ordering and pagination retain their v13s behavior.
Console must use one coherent vulnerability read/write flow, not combine v0s
findings with v13s summaries or assessments. v0s reads never fall back to v13s.

This PR only extends v0s reads. Shared vulnerability Issues still use v13s.
Switching `MISSING_SBOM`, `VULNERABLE_IMAGE` and
`EXTERNAL_INGRESS_URGENT_VULNERABILITY` to v0s is separate follow-up work.
`VulnerableImageIssue.riskScore` is unchanged here; removing it is a separate
breaking change coordinated with Console. Do not mix those legacy Issues with
v0s results in a Console view before that source switch.

## Implementation gaps

| Capability                       | Current implementation                                                                                                    | Required work                                                     |
| -------------------------------- | ------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------- |
| Findings and advisory details    | Existing v0s resolvers, including workload scopes, EPSS, KEV, fixed version and assessments                               | Extend these fields, not another finding API                      |
| Finding pagination               | Forward-only, server-side severity/identifier ordering; filter/order/workload-bound cursors                               | Reset old finding cursors when deploying this integration         |
| Finding counts/facets            | Full-selection facets and summaries; shared team/environment/label selection, distinct vulnerability/workload totals      | Verify deployed backend conformance                               |
| Coverage                         | Complete API inventory in workload facets and tenant/team summaries                                                       | Verify deployed inventory reconciliation                          |
| Workload selection               | Global `TeamScopeInput` and environment/label filters; existing team/environment/workload scopes                          | No additional API selection contract needed                       |
| Workload/vulnerability overviews | Unique-vulnerability groups, affected-workload connection, batch-loaded workload counts and team unique-severity ordering | No additional backend release needed for CVE navigation           |
| Assessment writes                | Team-authorized, gated v0s batches                                                                                        | Durable activity-log delivery before enablement                   |
| Vulnerability Issues             | Existing producers use v13s                                                                                               | Switch the three producers and remove obsolete risk-score payload |
| Manual reanalysis                | No public v0s RPC                                                                                                         | Do not expose a working reanalysis control                        |

### GitHub contract status

- The client and service now live together in
  [nais/v0s](https://github.com/nais/v0s), with the lightweight client module at
  `github.com/nais/v0s/api`. Nais API pins `v0.11.0`; the old external
  `github.com/nais/v0s-api` module is no longer used.
- The published contract includes finding ordering, package-type filters/facets,
  paginated unique-vulnerability and workload overviews, shared workload selection,
  and vulnerability/coverage summaries. See
  [nais/v0s#10](https://github.com/nais/v0s/pull/10),
  [nais/v0s#11](https://github.com/nais/v0s/pull/11),
  [nais/v0s#12](https://github.com/nais/v0s/pull/12),
  [nais/v0s#13](https://github.com/nais/v0s/pull/13), and
  [nais/v0s#14](https://github.com/nais/v0s/pull/14).
  Finding ordering and package-type filters/facets are integrated in GraphQL.
  Unique-vulnerability groups and per-workload counts are integrated in GraphQL.
  Tenant/team summaries are integrated on the existing finding connection;
  the gap table describes remaining API adapter work, not missing backend reads.
- Identifier filtering always uses repeated `vulnerability_ids`, including for
  one ID. The old singular field is reserved. CVSS is absent from the v0s
  contract; legacy v13s fields are unchanged.
- Durable assessment-event delivery is not part of this pinned contract.
  Keep new assessment writes gated.

Workload enrichment uses the simplified overview contract from
[nais/v0s#16](https://github.com/nais/v0s/pull/16): finding filters and batch
selection, without affected-only selection, custom workload ordering or per-row
cursors. API follows `next_page_token` until the batch is complete. It still
checks the presence of `total_count` as an overview-support guard, including on
empty responses, but does not use it for UI pagination. The pinned server retains
this field; removing that guard requires a coordinated compatibility change.

Published contracts and source inspection do not verify deployed tenant
versions. Deploy compatible backend support before enabling new API features;
API fakes alone cannot establish backend conformance.

### CVE navigation: workload identity

The existing filtered workload aggregates can identify affected workloads without
enumerating findings: API can load aggregates for its metadata-selected inventory,
keep workloads with matching findings, and sort before pagination. This costs
O(selected workloads), not O(findings), and package duplicates do not add rows.

The pinned protobuf `WorkloadRef` and `WorkloadSelector` identify workloads only by
environment/team/name, matching the backend workload primary key. This is
consistent with the platform invariant: naiserator rejects resource generation
for an application if a job with the same name exists in the same team/environment,
and vice versa. Both directions have regression fixtures:

- [Application name check](https://github.com/nais/naiserator/blob/master/pkg/generators/application.go#L76)
- [Job name check](https://github.com/nais/naiserator/blob/master/pkg/generators/naisjob.go#L61)
- [Application conflict fixture](https://github.com/nais/naiserator/blob/master/pkg/resourcecreator/testdata/error_name_conflict.yaml)
- [Job conflict fixture](https://github.com/nais/naiserator/blob/master/pkg/resourcecreator/testdata/naisjob/error_name_conflict.yaml)

Workload type is therefore not a backend prerequisite for CVE navigation or
assessment keys. API should retain current-image reconciliation and existing
workload node identities. Defensive app/job collision tests do not establish a
supported deployment scenario and must not drive a backend identity migration.
No suppression flag or default needs to change for this reason.

## Selection and counting semantics

Current finding and summary scopes select team, environment and workload.
`Team.vulnerabilityFindings(workloadFilter:)` and `Team.workloads(filter:)`
share the authoritative API workload inventory and existing environment/label
matching. All labels are ANDed: an omitted value requires the key to exist;
an explicit empty string requires an empty value. Environment values are ORed.
Selection applies to finding rows, facets, summaries and nested vulnerability
groups before pagination. The v0s selection and unique-count sorting paths
accept only environments and labels from `TeamWorkloadsFilter`; other workload
predicates fail explicitly rather than silently invoking legacy SBOM semantics.
Legacy workload filtering remains unchanged.
Global `vulnerabilityFindings(workloadFilter:)` accepts the same environment/label
selection and the existing `TeamScopeInput`. `{memberOfViewer: true}` selects
the viewer's teams; explicit `teamSlugs` and membership-derived teams are a
union, intersected with environments and labels. It does not impose a new
membership requirement on ordinary reads. Without a team restriction, the full
API inventory remains selected. An explicit empty `teamSlugs: []`, or a viewer
with no memberships using `memberOfViewer: true`, selects no workloads, even
for an administrator. A restricted empty selection never becomes unrestricted.
These rules apply equally to findings, groups, facets, summary and
`affectedWorkloads`. Existing read authorization is unchanged.

Finding predicates select vulnerability IDs/aliases, package/PURL, package type,
severity, available fix, KEV and assessment presence. `suppressed` defaults to
`EXCLUDE`; `INCLUDE` and `ONLY` retain their existing meaning. For the existing
positive-only fix/KEV predicates, false or omission means no restriction, not
"only without".

All counts and facets describe the full filtered selection, not the fetched
page, including when a page is empty:

- **Finding count:** one vulnerability/package finding in one workload's current
  image. Several PURLs can contribute several findings for one vulnerability.
  Findings in a shared image count separately for each affected workload.
- **Unique vulnerability count:** distinct canonical vulnerability identifiers.
  Aliases do not add another vulnerability.
- **Affected workload count:** distinct current workloads with matching findings.
- **Severity counts:** finding facets count findings. Summary and workload
  `vulnerabilityOverview.countsBySeverity` count unique canonical vulnerabilities,
  each once at its highest recognized severity among matching findings (UNKNOWN
  if all are UNKNOWN). The summary deduplicates across the whole selection;
  workload rows deduplicate within that workload. These counts need not add up
  across rows.
- **KEV counts:** explicitly distinguish findings, unique vulnerabilities and
  unique workloads. A `CountFindings` vulnerability bucket is a finding count,
  not a distinct workload count.
- **Coverage:** counts all workloads matching workload predicates, including
  clean and pending workloads. Finding predicates and affected-only filtering
  do not shrink this denominator. Missing/stale scans do not mean clean.

Workload lists remain authoritative in API and retain their existing filters
and pagination. Each workload exposes `vulnerabilityOverview(filter:)`; the
finding filter changes its counts, not membership in the workload list.
The request-scoped loader fetches server-aggregated counts in batches for the
requested workload identities, grouped by effective finding filter.
API workload identities and overview caches distinguish application from job,
even when name, team and environment are identical. Each item is reconciled
against its own deployed image; a backend record for the other item's image
cannot provide its counts or scan status. Backend selectors remain deduplicated
by environment/team/name, while summary coverage counts each selected API identity.
Workloads not yet observed by v0s, or whose deployed image reference has changed,
remain in the list as PENDING with zero known findings. Those zeroes do not mean
clean. Backend errors and unsupported overview responses are errors, not pending
fallbacks. A matching workload's coverage is independent of its finding filter.
`Team.workloads` supports `VULNERABILITY_UNIQUE_CRITICAL`, `HIGH`, `MEDIUM`,
`LOW` and `UNKNOWN` order fields (all with the `VULNERABILITY_UNIQUE_` prefix).
These sort on the corresponding unique-severity bucket, using the same
`filter.vulnerabilityFilter` supplied to each row's `vulnerabilityOverview(filter:)`.
This filter changes counts, not membership or coverage. Passing it with a
legacy or omitted order field is an error. Existing `VULNERABILITY_SEVERITY_*`
ordering continues to use v13s and must not be used in a v0s view.
`Team.vulnerabilityFindings.workloadFilter` still accepts only environments and
labels; put finding predicates in that field's existing `filter` argument.
`vulnerabilityFindings.affectedWorkloads` is the affected-only list for both
global and team CVE navigation. Unlike the complete workload list, it includes
only workloads with at least one matching current-image finding.

Package types are PURL ecosystems. Facets follow the complete filter, including
any selected package type, consistently with the existing finding facets.

## Ordering and pagination

Finding, vulnerability-group, affected-workload and suppression connections use `first`/`after`,
defaulting to 20 with a range of 1..200.
Legacy v13s connections retain their existing bidirectional pagination.

Omitting finding `orderBy` preserves environment, team, workload, identifier
and package URL ascending. Set `orderBy: {field: SEVERITY, direction: DESC}`
for CRITICAL, HIGH, MEDIUM, LOW, UNKNOWN. Ascending is LOW, MEDIUM, HIGH, CRITICAL,
UNKNOWN. UNKNOWN is last in both directions. Severity ties use canonical
identifier ascending, then environment/team/workload/PURL ascending. Identifier
ordering uses bytewise, case-sensitive collation with the same workload/PURL
identity tie-breakers. Both fields support ASC and DESC.

Sort the entire selection on the server before limiting it. Cursors bind to
operation, normalized filter, resolved workload/team selection and effective
ordering. Reject mismatches; changing filters or ordering starts a new traversal.
The finding adapter passes an explicit current-workload selector to the backend,
so package facets exclude removed workloads and stale deployed images before
pagination. Empty selections remain explicitly empty, not unrestricted.
Finding and vulnerability-group cursors issued before this integration must be discarded; restart
without a cursor. Changes to the resolved workload selection also require a restart.
Deterministic ordering does not imply a snapshot across concurrent updates.
Do not fetch all findings in Console or API to emulate sorting or grouping.

### Affected-workload pagination

`affectedWorkloads` inherits the parent workload selection and finding filter,
including vulnerability IDs/aliases and `suppressed`. Its own `first`/`after`
is independent of the parent finding page and order. The server filters and
deduplicates workloads before pagination; multiple matching packages produce
one row. `pageInfo.totalCount` is the full affected-workload total, not the
finding count. Rows are existing Workload nodes with stable `id` and `__typename`.

The fixed order is name, environment, team and workload type ascending, using
bytewise case-sensitive comparison. Cursors bind that order, effective scope,
resolved team selection, normalized metadata/finding filters and selected
workload identities/deployed images. Changing a filter, scope or deployed
selection requires a fresh traversal. Equivalent reordered/deduplicated
metadata filters are accepted. A deleted/nonmatching sort boundary can still
be resumed; this is not snapshot pagination.

API follows all backend workload-aggregate continuation pages for the selected
inventory, keeps positive filtered finding counts, then sorts and paginates.
It does not enumerate finding pages to calculate membership. Cost is
O(selected workloads), not backend-sized affected-only pagination. The pinned
v0.11.0 contract is sufficient; no nais/v0s dependency change is required.
`facets.vulnerabilityCoverage` covers the entire metadata-selected inventory,
just like the parent summary, not just affected rows. Missing/stale scans
remain pending and are never inferred clean.

### Unique-severity workload ordering

Workloads without a completed scan (PENDING, NO_SBOM, FAILED and
UNRESOLVED_IMAGE) come **first in both ASC and DESC**, ordered by name,
environment and workload type ascending. Missing or obsolete backend inventory
remains PENDING. These workloads are not treated as measured zero-count clean
workloads. SCANNED workloads then sort by the selected unique-severity count,
with the same ascending identity tie-breakers in either direction. Scanned
zero-count workloads participate normally in the completed-scan group.
Default NAME ASC and all legacy ordering retain their behavior.

Filtering and aggregation precede sorting and pagination. API bulk-loads v0s
aggregates for all selected team workloads, follows backend continuation and
shares the request cache with row overviews. It does not enumerate findings.
The cost is proportional to the selected team workload count; this is not
backend-sized count-sorted pagination.

The workload connection keeps its existing `first`/`after` and `last`/`before`
semantics. New unique-count modes issue versioned cursors bound to team,
normalized environments/labels, finding filter, severity and direction.
Equivalent reordered/deduplicated filters and different page sizes are accepted;
key-only labels remain distinct from explicit empty values. Changed selections
or ordering, unsupported cursor versions and old unbound cursors are rejected.
Legacy modes retain their offset cursors and reject new-mode cursors.
Positions are live offsets, not a snapshot: changed counts or inventory between
requests can shift page boundaries. Restart a traversal after such changes.

No nais/v0s change or new client release is required: the pinned `v0.11.0`
already supports explicit selectors, filtered workload aggregates and summary
coverage independent of finding filters.

## Additive GraphQL navigation

Implemented without replacing legacy root queries:

- `vulnerabilityFindings.vulnerabilities`: paginated canonical vulnerability groups.
- `vulnerabilityFindings.affectedWorkloads`: paginated current API workloads
  with matching findings, globally and in existing scoped finding connections.
- `vulnerabilityFindings.summary`: full-selection finding, unique-vulnerability
  and workload counts, KEV counts and coverage. Available globally and through
  team/environment scopes without separate summary APIs.
- `Workload.vulnerabilityOverview`: server-aggregated counts and current-image
  coverage on existing workload lists, applications and jobs.

The vulnerability connection inherits the full parent scope and finding filter,
independently of the parent finding page and ordering. Its default order is
severity DESC, UNKNOWN last, then identifier ASC; it also supports identifier
and affected-workload-count ASC/DESC. Cursors bind that effective order, filter
and workload selection. Omitted order and explicit severity DESC are equivalent.
The full unique-vulnerability total comes from v0s, even on empty pages.
Overview rows are derived scoped statistics, not separate nodes; existing
advisory and workload nodes retain their identities.

Summary severity buckets count each canonical vulnerability once, at its highest
known matching severity across the full selection. These totals are computed
directly in v0s, never by summing per-workload unique counts or finding pages.
Finding filters do not narrow summary coverage: API inventory supplies the
denominator, matching v0s inventory supplies scan categories, and missing or
stale-image workloads add PENDING. Unresolved images remain a separate category.
The categories sum to the API workload total. Backend errors and missing summary
data fail explicitly. A changed backend workload total during reconciliation is
an error requiring retry; the two services do not share an inventory snapshot.

Advisory details still use `vulnerability(identifier:)`. Use that same canonical
ID or alias in the existing finding filter to navigate to affected workloads;
there is no parallel advisory or finding API. Complete workload lists still
come from API rather than the asynchronously updated v0s inventory.

```graphql
query Overviews(
	$team: Slug!
	$environments: [String!]
	$labels: [LabelFilter!]
	$findings: VulnerabilityFindingFilter
	$after: Cursor
	$workloadAfter: Cursor
) {
	team(slug: $team) {
		vulnerabilityFindings(
			workloadFilter: { environments: $environments, labels: $labels }
			filter: $findings
		) {
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
			vulnerabilities(
				first: 20
				after: $after
				orderBy: { field: AFFECTED_WORKLOAD_COUNT, direction: DESC }
			) {
				nodes {
					vulnerabilityIdentifier
					severity
					findingCount
					affectedWorkloadCount
					vulnerability {
						title
						knownExploited
						epss {
							score
							date
						}
					}
				}
				pageInfo {
					totalCount
					endCursor
					hasNextPage
				}
			}
		}
		workloads(
			first: 20
			after: $workloadAfter
			filter: { environments: $environments, labels: $labels, vulnerabilityFilter: $findings }
			orderBy: { field: VULNERABILITY_UNIQUE_CRITICAL, direction: DESC }
		) {
			nodes {
				name
				vulnerabilityOverview(filter: $findings) {
					findingCount
					uniqueVulnerabilityCount
					knownExploitedVulnerabilityCount
					countsBySeverity {
						critical
						high
						medium
						low
						unknown
					}
					coverage {
						status
						scannedAt
						error
					}
				}
			}
			pageInfo {
				totalCount
				endCursor
				hasNextPage
			}
		}
	}
}
```

Example variables for the same workload and finding selection in the summary,
sort and row counts:

```json
{
	"team": "my-team",
	"environments": ["prod"],
	"labels": [{ "key": "group", "value": "core" }],
	"findings": { "packageType": "npm", "onlyWithFix": true }
}
```

Coverage includes every matching workload, even with no matching findings or
no completed scan. The finding and workload page sizes do not change summary
counts or coverage.

Existing advisory and package-finding queries remain available:

```graphql
query Advisory {
	vulnerability(identifier: "CVE-2024-12345") {
		identifier
		title
		description
		references
		knownExploited
		epss {
			score
			date
		}
	}
}
```

```graphql
query Packages($after: Cursor) {
	team(slug: "my-team") {
		environment(name: "dev") {
			workload(name: "my-app") {
				vulnerabilityCoverage {
					status
					scannedAt
					error
				}
				vulnerabilityFindings(
					first: 25
					after: $after
					filter: { vulnerabilityIdentifiers: ["CVE-2024-12345"], suppressed: INCLUDE }
				) {
					nodes {
						vulnerabilityIdentifier
						purl
						packageKey
						installedVersion
						fixedVersion
						severity
						severitySource
						knownExploited
						suppression {
							state
							reason
							updatedBy
							updatedAt
						}
					}
					pageInfo {
						totalCount
						endCursor
						hasNextPage
					}
				}
			}
		}
	}
}
```

## CVE navigation examples

### Global CVE list

Use the existing finding connection as a shared selection container. Console
does not need to request finding rows to render this list:

```graphql
query GlobalCves(
	$selection: VulnerabilityWorkloadFilter
	$filter: VulnerabilityFindingFilter
	$after: Cursor
) {
	vulnerabilityFindings(workloadFilter: $selection, filter: $filter) {
		summary {
			findingCount
			uniqueVulnerabilityCount
			affectedWorkloadCount
			knownExploitedVulnerabilityCount
			coverage {
				totalWorkloads
				scanned
				pending
				noSbom
				failed
				unresolvedImage
			}
		}
		facets {
			packageTypes {
				value
				count
			}
		}
		vulnerabilities(first: 20, after: $after) {
			nodes {
				vulnerabilityIdentifier
				severity
				findingCount
				affectedWorkloadCount
				vulnerability {
					identifier
					knownExploited
					epss {
						score
						date
					}
				}
			}
			pageInfo {
				totalCount
				endCursor
				hasNextPage
			}
		}
	}
}
```

```json
{
	"selection": {
		"teamScope": { "memberOfViewer": true },
		"environments": ["dev"],
		"labels": [{ "key": "group", "value": "core" }]
	},
	"filter": { "packageType": "npm", "suppressed": "INCLUDE" }
}
```

### Global and team CVE details

The same details fragment works on global and team finding connections.
`$filter` must contain the selected CVE's canonical ID or alias. Repeat the
finding filter on row reads: child workload connections do not implicitly
inherit a parent's arguments. Paginate their package findings separately if
the selected rows contain more than one finding page.

```graphql
fragment CveWorkloads on VulnerabilityFindingConnection {
	summary {
		findingCount
		uniqueVulnerabilityCount
		affectedWorkloadCount
		coverage {
			totalWorkloads
			scanned
			pending
			noSbom
			failed
			unresolvedImage
		}
	}
	affectedWorkloads(first: 20, after: $after) {
		nodes {
			id
			__typename
			name
			teamEnvironment {
				team {
					slug
				}
				environment {
					name
				}
			}
			vulnerabilityFindings(first: 100, filter: $filter) {
				nodes {
					vulnerabilityIdentifier
					image {
						name
					}
					packageKey
					purl
					fixedVersion
					suppression {
						id
						state
						reason
						updatedBy
						updatedAt
					}
				}
				pageInfo {
					totalCount
					endCursor
					hasNextPage
				}
			}
		}
		pageInfo {
			totalCount
			endCursor
			hasNextPage
		}
		facets {
			vulnerabilityCoverage {
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

query GlobalCveDetails(
	$identifier: String!
	$selection: VulnerabilityWorkloadFilter
	$filter: VulnerabilityFindingFilter!
	$after: Cursor
) {
	vulnerability(identifier: $identifier) {
		identifier
		title
		description
		references
		knownExploited
		epss {
			score
			date
		}
	}
	vulnerabilityFindings(workloadFilter: $selection, filter: $filter) {
		...CveWorkloads
	}
}

query TeamCveDetails(
	$team: Slug!
	$identifier: String!
	$selection: TeamWorkloadsFilter
	$filter: VulnerabilityFindingFilter!
	$after: Cursor
) {
	vulnerability(identifier: $identifier) {
		identifier
		title
		description
		references
		knownExploited
		epss {
			score
			date
		}
	}
	team(slug: $team) {
		vulnerabilityFindings(workloadFilter: $selection, filter: $filter) {
			...CveWorkloads
		}
	}
}
```

For global details, reuse the global list's `$selection` and set
`"identifier": "CVE-2024-12345"` and
`"filter": {"vulnerabilityIdentifiers": ["CVE-2024-12345"], "suppressed": "INCLUDE"}`.
For team details, supply `"team": "my-team"` and only `environments`/`labels`
inside `$selection`. An alias is also accepted; returned finding identifiers
are canonical. `INCLUDE` lets the UI show assessed and unassessed packages;
`ONLY`/`EXCLUDE` select assessed/unassessed findings consistently in affected
rows, groups, facets and counts. They never narrow coverage.

## Assessments

### Bulk assessments for selected rows

There is no single assessment per CVE/workload row: each matching package and
repository has its own assessment. Read `suppression` on the selected rows'
findings, including subsequent package pages where needed. Build keys from
the workload's team/environment/name and each finding's `image.name`,
`packageKey` and canonical `vulnerabilityIdentifier`. Do not send a workload
node ID or image tag/digest as an assessment key. Deduplicate identical keys
and group selected rows into separate owning-team batches.

Use only the existing mutations, when writes are enabled:

```graphql
mutation AssessSelectedPackages($input: SetVulnerabilitySuppressionsInput!) {
	setVulnerabilitySuppressions(input: $input) {
		suppressions {
			id
			imageName
			packageKey
			state
			reason
		}
	}
}

mutation DeleteSelectedAssessments($input: DeleteVulnerabilitySuppressionsInput!) {
	deleteVulnerabilitySuppressions(input: $input) {
		deleted {
			environment
			workload
			imageName
			packageKey
			vulnerabilityIdentifier
		}
	}
}
```

Example set input (delete uses the same keys in `input.keys`):

```json
{
	"input": {
		"team": "my-team",
		"items": [
			{
				"key": {
					"environment": "dev",
					"workload": "my-app",
					"imageName": "registry.example/my-app",
					"packageKey": "pkg:npm/example",
					"vulnerabilityIdentifier": "CVE-2024-12345"
				},
				"state": "ACCEPTED_RISK",
				"reason": "Reviewed by the owning team"
			}
		]
	}
}
```

After a successful set/delete, refetch the v0s selection and discard affected
cursors if the selected filter changes. An `EXCLUDE` view may lose a row once
all its matching packages are assessed; `ONLY` may gain it. Existing team
authorization, atomic per-team batches and activity-log behavior remain
unchanged. This API change neither enables `V0S_SUPPRESSIONS_ENABLED` nor
changes its default. Disabled writes remain disabled; do not use
`updateImageVulnerability` as a write fallback for a v0s view.

### Existing assessment guarantees and rollout constraints

New assessments use only NOT_AFFECTED, FALSE_POSITIVE and ACCEPTED_RISK.
Their key is `(team, environment, workload, image repository, versionless
package key, canonical vulnerability ID)`. They survive image version changes,
but are not automatically shared across workloads or repositories.

Writes use `vulnerability:update` authorization for the whole owning-team
batch. The authenticated actor is supplied by API, not user input. v0s
canonicalizes keys and commits each set/delete batch atomically.

Currently activity-log entries are written separately after the v0s commit.
This does not guarantee delivery: a log failure can occur after a successful
assessment write. Before enabling new writes, the required backend extension
must commit immutable change events in the same transaction as the assessment.
API must consume those events idempotently, committing deduplication and the
activity entry together before acknowledging delivery. Preserve original
actor, action, key, state, reason and timestamp, including deletion snapshots.

Importing old v13s assessments is a separate rollout responsibility. Do not
silently map RESOLVED or IN_TRIAGE to ACCEPTED_RISK. v0s's internal migration of
its own legacy assessment table is not a v13s import.

## Issues and reanalysis

The planned v0s Urgent producer will combine unsuppressed KEV data with existing confirmed external
exposure logic for applications; KEV alone is not Urgent. Jobs should retain
the general known-exploited-image issue. Issue counts must state their unit.
Only NO_SBOM means missing SBOM; pending, failed and unresolved are different.

Replace old vulnerability-derived issue rows before advertising v0s issue
readiness. Do not fall back to v13s on errors or clear vulnerability Issues in a
way that falsely reports a clean state. Other issue families remain unchanged.

v0s automatically reanalyses stored SBOMs when its Trivy database changes.
Restarting v0s does not force reanalysis. No manual-reanalysis RPC is currently
available; Console must not present one as functional.

## Required rollout order

1. Publish and deploy backend support for ordering, shared current-workload/image
   selection, package-type facets, paginated groups, distinct totals and correct
   multiple-ID filtering. Verify actual server behavior.
2. Deploy additive API fields and explicit readiness information. Keep legacy
   v13s APIs operational; no implicit fallback between models.
3. Deploy transactional assessment events and replay-safe API log ingestion.
   Keep new assessment writes disabled until delivery guarantees are verified.
4. Handle any old assessment import externally, including unmatched records and
   incompatible states. Any importer must not overwrite subsequent v0s writes.
5. Switch the three shared vulnerability issue producers to v0s and verify their
   refreshed data, including mixed-version rollout behavior.
6. Switch Console to the coherent v0s read/write flow. Remove the obsolete
   riskScore issue fragment and legacy vulnerability fragments.
7. Leave legacy v13s queries and mutations available for legacy consumers.
   Eventual removal is a separate change.

Rolling Console back does not copy newer v0s assessments into v13s. Preserve
assessment state and undelivered log events; explicitly reconcile any data
transfer instead of claiming consistency between independent models.
