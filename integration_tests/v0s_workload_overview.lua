Config.V0s = true
local user = User.new()
Team.new("v0s-overview", "Shared vulnerability selection", "#v0s")
Helper.readK8sResources("k8s_resources/v0s_overview")

local labels = [[
[
	{key: "group", value: "core"}, {key: "present"}, {key: "empty", value: ""}
]
]]
local workloads = string.format([[{environments: ["dev"], labels: %s}]], labels)
local findings = [[{packageType: "npm", onlyWithFix: true}]]
local filteredWorkloads = string.format([[{environments: ["dev"], labels: %s, vulnerabilityFilter: %s}]], labels,
	findings)

local function row(name, status, findingCount, uniqueCount)
	return {
		name = name,
		vulnerabilityOverview = {
			findingCount = findingCount or 0,
			uniqueVulnerabilityCount = uniqueCount or 0,
			countsBySeverity = { critical = uniqueCount or 0, high = 0 },
			coverage = { status = status },
		},
	}
end

Test.gql("Summary and sorted workloads share environment and AND label selection before pagination", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[{
		team(slug: "v0s-overview") {
			vulnerabilityFindings(first: 1, workloadFilter: %s, filter: %s) {
				nodes { vulnerabilityIdentifier }
				summary {
					findingCount uniqueVulnerabilityCount affectedWorkloadCount
					knownExploitedVulnerabilityCount knownExploitedWorkloadCount
					countsBySeverity { critical high }
					coverage { totalWorkloads scanned pending noSbom failed unresolvedImage }
				}
			}
			workloads(first: 6, filter: %s,
				orderBy: {field: VULNERABILITY_UNIQUE_CRITICAL, direction: DESC}) {
				nodes {
					name
					vulnerabilityOverview(filter: %s) {
						findingCount uniqueVulnerabilityCount countsBySeverity { critical high } coverage { status }
					}
				}
				pageInfo { totalCount endCursor hasNextPage }
				facets { vulnerabilityCoverage { totalWorkloads scanned pending noSbom failed unresolvedImage } }
			}
		}
	}]], workloads, findings, filteredWorkloads, findings))
	local coverage = { totalWorkloads = 8, scanned = 3, pending = 2, noSbom = 1, failed = 1, unresolvedImage = 1 }
	t.check { data = { team = {
		vulnerabilityFindings = {
			nodes = { { vulnerabilityIdentifier = "CVE-shared" } },
			summary = {
				findingCount = 3, uniqueVulnerabilityCount = 1, affectedWorkloadCount = 2,
				knownExploitedVulnerabilityCount = 1, knownExploitedWorkloadCount = 2,
				countsBySeverity = { critical = 1, high = 0 }, coverage = coverage,
			},
		},
		workloads = {
			nodes = {
				row("failed", "FAILED"), row("fresh", "PENDING"), row("no-sbom", "NO_SBOM"),
				row("stale", "PENDING"), row("unresolved", "UNRESOLVED_IMAGE"), row("alpha", "SCANNED", 2, 1),
			},
			pageInfo = { totalCount = 8, endCursor = Save("v0s_workload_cursor"), hasNextPage = true },
			facets = { vulnerabilityCoverage = coverage },
		},
	} } }

	t.query(string.format([[{
		team(slug: "v0s-overview") {
			workloads(first: 1, after: "%s", filter: %s,
				orderBy: {field: VULNERABILITY_UNIQUE_CRITICAL, direction: DESC}) {
				nodes {
					name
					vulnerabilityOverview(filter: %s) {
						findingCount uniqueVulnerabilityCount countsBySeverity { critical high } coverage { status }
					}
				}
				pageInfo { totalCount endCursor hasNextPage pageStart }
			}
		}
	}]], State.v0s_workload_cursor, filteredWorkloads, findings))
	t.check { data = { team = { workloads = {
		nodes = { row("zeta", "SCANNED", 1, 1) },
		pageInfo = { totalCount = 8, endCursor = Save("v0s_zeta_cursor"), hasNextPage = true, pageStart = 7 },
	} } } }
	t.query(string.format([[{
		team(slug: "v0s-overview") {
			workloads(first: 1, after: "%s", filter: %s,
				orderBy: {field: VULNERABILITY_UNIQUE_CRITICAL, direction: DESC}) {
				nodes { name } pageInfo { hasNextPage }
			}
		}
	}]], State.v0s_zeta_cursor, filteredWorkloads))
	t.check { data = { team = { workloads = { nodes = { { name = "clean" } }, pageInfo = { hasNextPage = false } } } } }

	t.query(string.format([[{
		team(slug: "v0s-overview") {
			workloads(first: 1, after: "%s",
				filter: {environments: ["dev"], labels: %s, vulnerabilityFilter: {packageType: "golang"}},
				orderBy: {field: VULNERABILITY_UNIQUE_CRITICAL, direction: DESC}) { nodes { name } }
		}
	}]], State.v0s_workload_cursor, labels))
	t.check { data = Null, errors = { {
		message = "Cursor does not belong to this workload vulnerability selection; restart without a cursor",
		path = { "team", "workloads" }, locations = NotNull(),
	} } }
end)

Test.gql("Ascending keeps incomplete scans first and finding filters do not shrink coverage", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[{
		team(slug: "v0s-overview") {
			workloads(filter: %s,
				orderBy: {field: VULNERABILITY_UNIQUE_CRITICAL, direction: ASC}) { nodes { name } }
			vulnerabilityFindings(workloadFilter: %s, filter: {packageType: "absent"}) {
				summary { findingCount uniqueVulnerabilityCount affectedWorkloadCount coverage { totalWorkloads scanned pending } }
			}
			empty: vulnerabilityFindings(workloadFilter: {environments: ["dev"], labels: [{key: "absent"}]}) {
				nodes { vulnerabilityIdentifier } summary { findingCount coverage { totalWorkloads } }
			}
			emptyWorkloads: workloads(filter: {environments: ["dev"], labels: [{key: "absent"}]},
				orderBy: {field: VULNERABILITY_UNIQUE_CRITICAL, direction: DESC}) {
				nodes { name } pageInfo { totalCount }
			}
		}
	}]], filteredWorkloads, workloads))
	t.check { data = { team = {
		workloads = { nodes = {
			{ name = "failed" }, { name = "fresh" }, { name = "no-sbom" }, { name = "stale" },
			{ name = "unresolved" }, { name = "clean" }, { name = "alpha" }, { name = "zeta" },
		} },
		vulnerabilityFindings = { summary = {
			findingCount = 0, uniqueVulnerabilityCount = 0, affectedWorkloadCount = 0,
			coverage = { totalWorkloads = 8, scanned = 3, pending = 2 },
		} },
		empty = { nodes = {}, summary = { findingCount = 0, coverage = { totalWorkloads = 0 } } },
		emptyWorkloads = { nodes = {}, pageInfo = { totalCount = 0 } },
	} } }
end)

Test.gql("New vulnerability selection rejects unsupported workload predicates and legacy ordering", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{
		team(slug: "v0s-overview") {
			vulnerabilityFindings(workloadFilter: {vulnerabilityCoverageStatuses: [SCANNED]}) { summary { findingCount } }
		}
	}]]
	t.check { data = Null, errors = { {
		message = "Only environments and labels are supported for vulnerability workload selection",
		path = { "team", "vulnerabilityFindings" }, locations = NotNull(),
	} } }
	t.query [[{
		team(slug: "v0s-overview") {
			workloads(filter: {vulnerabilityFilter: {packageType: "npm"}}, orderBy: {field: NAME, direction: ASC}) { nodes { name } }
		}
	}]]
	t.check { data = Null, errors = { {
		message = "filter.vulnerabilityFilter requires a VULNERABILITY_UNIQUE workload order field",
		path = { "team", "workloads" }, locations = NotNull(),
	} } }
	t.query [[{
		team(slug: "v0s-overview") {
			vulnerabilityFindings(workloadFilter: {vulnerabilityFilter: {packageType: "npm"}}) { summary { findingCount } }
		}
	}]]
	t.check { data = Null, errors = { {
		message = "Use filter to select vulnerability findings; workloadFilter only supports environments and labels",
		path = { "team", "vulnerabilityFindings" }, locations = NotNull(),
	} } }
end)

Test.gql("Same-name application and job keep their own images and coverage", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{
		team(slug: "v0s-overview") {
			vulnerabilityFindings(workloadFilter: {environments: ["dev"], labels: [{key: "group", value: "collision"}]}) {
				summary { findingCount coverage { totalWorkloads scanned pending } }
			}
			appOnly: vulnerabilityFindings(workloadFilter: {environments: ["dev"], labels: [{key: "owner", value: "app"}]}) {
				nodes { vulnerabilityIdentifier }
				summary { findingCount coverage { totalWorkloads scanned pending } }
			}
			workloads(filter: {environments: ["dev"], labels: [{key: "group", value: "collision"}]},
				orderBy: {field: VULNERABILITY_UNIQUE_CRITICAL, direction: ASC}) {
				nodes {
					__typename name
					vulnerabilityOverview { findingCount countsBySeverity { critical } coverage { status } }
				}
				facets { vulnerabilityCoverage { totalWorkloads scanned pending } }
			}
		}
	}]]
	local coverage = { totalWorkloads = 2, scanned = 1, pending = 1 }
	t.check { data = { team = {
		vulnerabilityFindings = { summary = { findingCount = 1, coverage = coverage } },
		appOnly = { nodes = {}, summary = { findingCount = 0, coverage = { totalWorkloads = 1, scanned = 0, pending = 1 } } },
		workloads = {
			nodes = {
				{ __typename = "Application", name = "collision",
					vulnerabilityOverview = { findingCount = 0, countsBySeverity = { critical = 0 }, coverage = { status = "PENDING" } } },
				{ __typename = "Job", name = "collision",
					vulnerabilityOverview = { findingCount = 1, countsBySeverity = { critical = 1 }, coverage = { status = "SCANNED" } } },
			},
			facets = { vulnerabilityCoverage = coverage },
		},
	} } }
end)
