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

| Capability                       | Current implementation                                                                      | Required work                                                                      |
| -------------------------------- | ------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| Findings and advisory details    | Existing v0s resolvers, including workload scopes, EPSS, KEV, fixed version and assessments | Extend these fields, not another finding API                                       |
| Finding pagination               | Forward-only, server-side severity/identifier ordering; filter/order/workload-bound cursors | Reset old finding cursors when deploying this integration                          |
| Finding counts/facets            | Full-selection facets and tenant/team summaries; distinct vulnerability/workload totals     | Shared global workload filtering                                                   |
| Coverage                         | Complete API inventory in workload facets and tenant/team summaries                         | Verify deployed inventory reconciliation                                           |
| Workload selection               | Team/environment/workload scopes; existing workload environment/label filters               | Shared global selection and existing `TeamScopeInput` support                      |
| Workload/vulnerability overviews | Unique-vulnerability connection; existing API workload pages with batch-loaded v0s counts   | Global affected-only workload selection; v0s count-based workload sorting deferred |
| Assessment writes                | Team-authorized, gated v0s batches                                                          | Durable activity-log delivery before enablement                                    |
| Vulnerability Issues             | Existing producers use v13s                                                                 | Switch the three producers and remove obsolete risk-score payload                  |
| Manual reanalysis                | No public v0s RPC                                                                           | Do not expose a working reanalysis control                                         |

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

## Selection and counting semantics

Current finding and summary scopes select team, environment and workload.
Existing API workload lists retain their environment and label filters, with
v0s counts available on each returned workload. Shared label and "my teams"
selection for global findings, groups and summaries is not implemented in this
PR; it must reuse existing `TeamScopeInput` and label matching in follow-up work.
A restricted empty selection must never become unrestricted.

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
- **Severity counts:** finding counts unless explicitly named as unique
  vulnerability counts. A grouped vulnerability takes the highest recognized
  severity among its filtered findings, or UNKNOWN if all are UNKNOWN.
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
Workloads not yet observed by v0s, or whose deployed image reference has changed,
remain in the list as PENDING with zero known findings. Those zeroes do not mean
clean. Backend errors and unsupported overview responses are errors, not pending
fallbacks. A matching workload's coverage is independent of its finding filter.
v0s finding-count ordering for workload lists is deliberately deferred, and the
global affected-only workload list is not integrated yet. Existing legacy
vulnerability ordering continues to use v13s and must not be used in a v0s view.

Package types are PURL ecosystems. Facets follow the complete filter, including
any selected package type, consistently with the existing finding facets.

## Ordering and pagination

New v0s lists use `first`/`after`, defaulting to 20 with a range of 1..200.
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
Finding cursors issued before this integration must be discarded; restart
without a cursor. Changes to the resolved workload selection also require a restart.
Deterministic ordering does not imply a snapshot across concurrent updates.
Do not fetch all findings in Console or API to emulate sorting or grouping.

## Additive GraphQL navigation

Implemented without replacing legacy root queries:

- `vulnerabilityFindings.vulnerabilities`: paginated canonical vulnerability groups.
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

Direct advisory-to-affected-workload navigation
is not integrated yet. The earlier proposed parallel
`vulnerabilityFindings.workloads` list is not introduced: complete workload lists
come from API rather than the asynchronously updated v0s inventory.

```graphql
query Overviews($after: Cursor, $workloadAfter: Cursor) {
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
		workloads(first: 20, after: $workloadAfter) {
			nodes {
				name
				vulnerabilityOverview(filter: { packageType: "npm" }) {
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

## Assessments

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
