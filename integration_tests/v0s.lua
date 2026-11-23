Config.V0s = true
local user = User.new()
Team.new("v0s-team", "Vulnerability migration", "#v0s")
Helper.readK8sResources("k8s_resources/v0s")

Test.gql("Current findings use workload inventory and coexist with legacy advisories", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{
		vulnerability(identifier: "CVE-1") { id identifier references }
		cve(identifier: "CVE-2024-12345") { identifier }
		team(slug: "v0s-team") {
			vulnerabilityFindings(first: 1) {
				nodes { id vulnerabilityIdentifier severity fixedVersion workload { name } }
				edges { cursor }
				pageInfo { totalCount hasNextPage hasPreviousPage pageStart pageEnd endCursor }
				facets { fixableFindings knownExploitedFindings countsBySeverity { high critical } }
			}
			workloads(first: 1) { facets { vulnerabilityCoverage { totalWorkloads scanned pending } } }
			pending: workloads(filter: {vulnerabilityCoverageStatuses: [PENDING]}) {
				nodes { name __typename vulnerabilityCoverage { status imageDigest } }
			}
			environment(name: "dev") {
				vulnerabilityFindings(filter: {vulnerabilityIdentifiers: ["CVE-1", "missing"]}) {
					nodes { vulnerabilityIdentifier }
					pageInfo { totalCount }
				}
				workload(name: "app") {
					vulnerabilityFindings(filter: {onlyWithFix: true}) { pageInfo { totalCount } }
					vulnerabilityCoverage { status imageDigest }
				}
			}
		}
	}]]
	t.check { data = {
		vulnerability = { id = Save("advisory_id"), identifier = "CVE-1", references = { "https://example.com/advisories/CVE-1" } },
		cve = { identifier = "CVE-2024-12345" },
		team = {
			vulnerabilityFindings = {
				nodes = { { id = Save("finding_id"), vulnerabilityIdentifier = "CVE-1", severity = "HIGH", fixedVersion = Null, workload = { name = "app" } } },
				edges = { { cursor = Save("finding_cursor") } },
				pageInfo = { totalCount = 2, hasNextPage = true, hasPreviousPage = false, pageStart = 1, pageEnd = 1, endCursor = Save("end_cursor") },
				facets = { fixableFindings = 1, knownExploitedFindings = 1, countsBySeverity = { high = 1, critical = 1 } },
			},
			workloads = { facets = { vulnerabilityCoverage = { totalWorkloads = 3, scanned = 1, pending = 2 } } },
			pending = { nodes = {
				{ name = "fresh", __typename = "Job",         vulnerabilityCoverage = { status = "PENDING", imageDigest = Null } },
				{ name = "stale", __typename = "Application", vulnerabilityCoverage = { status = "PENDING", imageDigest = Null } },
			} },
			environment = {
				vulnerabilityFindings = { nodes = { { vulnerabilityIdentifier = "CVE-1" } }, pageInfo = { totalCount = 1 } },
				workload = { vulnerabilityFindings = { pageInfo = { totalCount = 1 } }, vulnerabilityCoverage = { status = "SCANNED", imageDigest = "sha256:app" } },
			},
		},
	} }
	assert(State.finding_cursor == State.end_cursor, "edge and page cursors differ")

	t.query(string.format([[{
		team(slug: "v0s-team") { vulnerabilityFindings(first: 1, after: "%s") {
			nodes { vulnerabilityIdentifier fixedVersion }
			pageInfo { totalCount hasNextPage hasPreviousPage pageStart pageEnd }
		} }
		finding: node(id: "%s") { ... on VulnerabilityFinding { vulnerabilityIdentifier vulnerability { identifier } } }
		advisory: node(id: "%s") { ... on Vulnerability { identifier } }
	}]], State.end_cursor, State.finding_id, State.advisory_id))
	t.check { data = {
		team = { vulnerabilityFindings = {
			nodes = { { vulnerabilityIdentifier = "CVE-2", fixedVersion = "2" } },
			pageInfo = { totalCount = 2, hasNextPage = false, hasPreviousPage = true, pageStart = 2, pageEnd = 2 },
		} },
		finding = { vulnerabilityIdentifier = "CVE-1", vulnerability = { identifier = "CVE-1" } },
		advisory = { identifier = "CVE-1" },
	} }
end)

Test.gql("Advisory lookup distinguishes ambiguous and missing identifiers", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ vulnerability(identifier: "ambiguous") { identifier } }]]
	t.check { data = { vulnerability = Null }, errors = { { locations = NotNull(), message = "alias names several vulnerabilities", path = { "vulnerability" } } } }
	t.query [[{ vulnerability(identifier: "missing") { identifier } }]]
	t.check { data = { vulnerability = Null } }
end)
