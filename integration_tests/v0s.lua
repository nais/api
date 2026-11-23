Config.V0s = true
local user = User.new()
Team.new("v0s-team", "Vulnerability migration", "#v0s")
Helper.readK8sResources("k8s_resources/v0s")

Test.gql("Current findings use workload inventory and coexist with legacy advisories", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{
		vulnerabilityFindings(filter: {scope: {team: "v0s-team"}}, first: 1) {
			nodes { id vulnerabilityIdentifier severity fixedVersion }
			edges { cursor }
			pageInfo { totalCount hasNextPage hasPreviousPage pageStart pageEnd endCursor }
		}
		vulnerabilityFindingSummary(filter: {scope: {team: "v0s-team"}}) {
			totalFindings fixableFindings knownExploitedFindings countsBySeverity { high critical }
		}
		filtered: vulnerabilityFindingSummary(filter: {scope: {team: "v0s-team"}, onlyWithFix: true}) { totalFindings }
		vulnerabilityCoverage(scope: {team: "v0s-team"}) { totalWorkloads scanned pending }
		vulnerability(identifier: "CVE-1") { id identifier cvss { score } }
		cve(identifier: "CVE-2024-12345") { identifier }
		team(slug: "v0s-team") { environment(name: "dev") {
			app: workload(name: "app") { vulnerabilityFindingSummary { totalFindings } vulnerabilityCoverage { status imageDigest } }
			stale: workload(name: "stale") { vulnerabilityCoverage { status imageDigest } }
			fresh: workload(name: "fresh") { __typename vulnerabilityCoverage { status } }
		} }
	}]]
	t.check { data = {
		vulnerabilityFindings = {
			nodes = { { id = Save("finding_id"), vulnerabilityIdentifier = "CVE-1", severity = "HIGH", fixedVersion = Null } },
			edges = { { cursor = Save("finding_cursor") } },
			pageInfo = { totalCount = 2, hasNextPage = true, hasPreviousPage = false, pageStart = 1, pageEnd = 1, endCursor = Save("end_cursor") },
		},
		vulnerabilityFindingSummary = { totalFindings = 2, fixableFindings = 1, knownExploitedFindings = 1, countsBySeverity = { high = 1, critical = 1 } },
		filtered = { totalFindings = 1 },
		vulnerabilityCoverage = { totalWorkloads = 3, scanned = 1, pending = 2 },
		vulnerability = { id = Save("advisory_id"), identifier = "CVE-1", cvss = { { score = Null }, { score = 0 } } },
		cve = { identifier = "CVE-2024-12345" },
		team = { environment = {
			app = { vulnerabilityFindingSummary = { totalFindings = 2 }, vulnerabilityCoverage = { status = "SCANNED", imageDigest = "sha256:app" } },
			stale = { vulnerabilityCoverage = { status = "PENDING", imageDigest = Null } },
			fresh = { __typename = "Job", vulnerabilityCoverage = { status = "PENDING" } },
		} },
	} }
	assert(State.finding_cursor == State.end_cursor, "edge and page cursors differ")

	t.query(string.format([[{
		vulnerabilityFindings(filter: {scope: {team: "v0s-team"}}, first: 1, after: "%s") {
			nodes { vulnerabilityIdentifier fixedVersion }
			pageInfo { totalCount hasNextPage hasPreviousPage pageStart pageEnd }
		}
		finding: node(id: "%s") { ... on VulnerabilityFinding { vulnerabilityIdentifier vulnerability { identifier } } }
		advisory: node(id: "%s") { ... on Vulnerability { identifier } }
	}]], State.end_cursor, State.finding_id, State.advisory_id))
	t.check { data = {
		vulnerabilityFindings = {
			nodes = { { vulnerabilityIdentifier = "CVE-2", fixedVersion = "2" } },
			pageInfo = { totalCount = 2, hasNextPage = false, hasPreviousPage = true, pageStart = 2, pageEnd = 2 },
		},
		finding = { vulnerabilityIdentifier = "CVE-1", vulnerability = { identifier = "CVE-1" } },
		advisory = { identifier = "CVE-1" },
	} }
end)

Test.gql("Assessment writes stay disabled and backend validation remains a client error", function(t)
	t.addHeader("x-user-email", user:email())
	for _, mutation in ipairs({
		[[setVulnerabilitySuppressions(input: {team: "v0s-team", items: [{key: {environment: "dev", workload: "app", imageName: "repo/app", packageKey: "pkg:npm/example", vulnerabilityIdentifier: "CVE-1"}, state: ACCEPTED_RISK, reason: "assessed"}]}) { suppressions { state } }]],
		[[deleteVulnerabilitySuppressions(input: {team: "v0s-team", keys: [{environment: "dev", workload: "app", imageName: "repo/app", packageKey: "pkg:npm/example", vulnerabilityIdentifier: "CVE-1"}]}) { deleted { state } }]],
	}) do
		t.query("mutation { " .. mutation .. " }")
		t.check { data = Null, errors = { { locations = NotNull(), message = Contains("writes are disabled during migration"), path = NotNull() } } }
	end
	t.query [[{ vulnerability(identifier: "ambiguous") { identifier } }]]
	t.check { data = { vulnerability = Null }, errors = { { locations = NotNull(), message = "alias names several vulnerabilities", path = { "vulnerability" } } } }
	t.query [[{ vulnerability(identifier: "missing") { identifier } }]]
	t.check { data = { vulnerability = Null } }
end)
