Config.V0s = true
local member = User.new()
local outsider = User.new()
local team = Team.new("v0s-overview", "CVE navigation", "#v0s")
team:addMember(member)
Team.new("v0s-team", "Other workloads", "#v0s")
Helper.readK8sResources("k8s_resources/v0s_overview")
Helper.readK8sResources("k8s_resources/v0s")

local metadata = [[{environments: ["dev"], labels: [
	{key: "group", value: "core"}, {key: "present"}, {key: "empty", value: ""}
]}]]
local globalMetadata = [[{teamScope: {memberOfViewer: true}, environments: ["dev"], labels: [
	{key: "group", value: "core"}, {key: "present"}, {key: "empty", value: ""}
]}]]
local findings = [[{vulnerabilityIdentifiers: ["GHSA-shared"], packageType: "npm",
	severities: [HIGH, CRITICAL], onlyWithFix: true, onlyKnownExploited: true, suppressed: INCLUDE}]]
local coverage = { totalWorkloads = 8, scanned = 3, pending = 2, noSbom = 1, failed = 1, unresolvedImage = 1 }
local fields = [[
	nodes { vulnerabilityIdentifier }
	pageInfo { totalCount }
	facets { countsBySeverity { high critical } packageTypes { value count } }
	summary { findingCount uniqueVulnerabilityCount affectedWorkloadCount
		countsBySeverity { critical high }
		coverage { totalWorkloads scanned pending noSbom failed unresolvedImage } }
	vulnerabilities(first: 1) {
		nodes { vulnerabilityIdentifier findingCount affectedWorkloadCount }
		pageInfo { totalCount }
	}
	affectedWorkloads(first: 1) {
		nodes {
			id name __typename
			vulnerabilityFindings(filter: {vulnerabilityIdentifiers: ["GHSA-shared"], packageType: "npm", suppressed: INCLUDE}) {
				nodes { vulnerabilityIdentifier packageKey suppression { state } }
			}
		}
		pageInfo { totalCount hasNextPage endCursor }
		facets { vulnerabilityCoverage { totalWorkloads scanned pending noSbom failed unresolvedImage } }
	}
]]

local function selectionResult()
	return {
		nodes = { { vulnerabilityIdentifier = "CVE-shared" } },
		pageInfo = { totalCount = 3 },
		facets = { countsBySeverity = { high = 1, critical = 2 }, packageTypes = { { value = "npm", count = 3 } } },
		summary = {
			findingCount = 3,
			uniqueVulnerabilityCount = 1,
			affectedWorkloadCount = 2,
			countsBySeverity = { critical = 1, high = 0 },
			coverage = coverage,
		},
		vulnerabilities = {
			nodes = { { vulnerabilityIdentifier = "CVE-shared", findingCount = 3, affectedWorkloadCount = 2 } },
			pageInfo = { totalCount = 1 },
		},
		affectedWorkloads = {
			nodes = { {
				id = Save("cve_alpha_id"),
				name = "alpha",
				__typename = "Application",
				vulnerabilityFindings = {
					nodes = {
						{ vulnerabilityIdentifier = "CVE-shared", packageKey = "pkg:npm/first",  suppression = Null },
						{ vulnerabilityIdentifier = "CVE-shared", packageKey = "pkg:npm/second", suppression = Null },
					},
				},
			} },
			pageInfo = { totalCount = 2, hasNextPage = true, endCursor = Save("cve_workload_cursor") },
			facets = { vulnerabilityCoverage = coverage },
		},
	}
end

Test.gql("My teams selects the full CVE flow, including package facets and affected workloads", function(t)
	t.addHeader("x-user-email", member:email())
	t.query(string.format([[{
		vulnerability(identifier: "GHSA-shared") { identifier }
		vulnerabilityFindings(first: 1, workloadFilter: %s, filter: %s) { %s }
	}]], globalMetadata, findings, fields))
	t.check { data = { vulnerability = { identifier = "CVE-shared" }, vulnerabilityFindings = selectionResult() } }
	local globalCursor = State.cve_workload_cursor
	local alphaID = State.cve_alpha_id

	t.query(string.format([[{
		team(slug: "v0s-overview") {
			vulnerabilityFindings(first: 1, workloadFilter: %s, filter: %s) { %s }
		}
	}]], metadata, findings, fields))
	t.check { data = { team = { vulnerabilityFindings = selectionResult() } } }
	assert(State.cve_alpha_id == alphaID, "workload identity changed with scope")

	t.query(string.format([[{
		vulnerabilityFindings(first: 1, workloadFilter: %s, filter: %s,
			orderBy: {field: IDENTIFIER, direction: DESC}) {
			affectedWorkloads(first: 1, after: "%s") {
				nodes { id name __typename }
				pageInfo { totalCount hasNextPage hasPreviousPage }
			}
		}
	}]], globalMetadata, findings, globalCursor))
	t.check { data = { vulnerabilityFindings = { affectedWorkloads = {
		nodes = { { id = Save("cve_zeta_id"), name = "zeta", __typename = "Job" } },
		pageInfo = { totalCount = 2, hasNextPage = false, hasPreviousPage = true },
	} } } }
	assert(State.cve_zeta_id ~= alphaID, "different workloads shared an identity")

	t.query(string.format([[{
		vulnerabilityFindings(workloadFilter: %s, filter: {vulnerabilityIdentifiers: ["GHSA-shared"], packageType: "golang"}) {
			affectedWorkloads(after: "%s") { nodes { name } }
		}
	}]], globalMetadata, globalCursor))
	t.check { data = Null, errors = { {
		message = "Cursor does not belong to this vulnerability selection",
		path = { "vulnerabilityFindings", "affectedWorkloads" }, locations = NotNull(),
	} } }

	t.query(string.format([[{
		vulnerabilityFindings(workloadFilter: {
			teamScope: {memberOfViewer: true}, environments: ["dev"],
			labels: [{key: "group", value: "core"}, {key: "present"}]
		}, filter: %s) {
			affectedWorkloads(after: "%s") { nodes { name } }
		}
	}]], findings, globalCursor))
	t.check { data = Null, errors = { {
		message = "Cursor does not belong to this vulnerability selection",
		path = { "vulnerabilityFindings", "affectedWorkloads" }, locations = NotNull(),
	} } }
end)

Test.gql("My teams without memberships and explicit empty selections stay empty", function(t)
	t.addHeader("x-user-email", outsider:email())
	t.query(string.format([[{
		vulnerabilityFindings(workloadFilter: %s, filter: %s) {
			nodes { vulnerabilityIdentifier } pageInfo { totalCount }
			vulnerabilities { nodes { vulnerabilityIdentifier } pageInfo { totalCount } }
			facets { packageTypes { value count } countsBySeverity { critical } }
			summary { findingCount affectedWorkloadCount coverage { totalWorkloads } }
			affectedWorkloads { nodes { name } pageInfo { totalCount } facets { vulnerabilityCoverage { totalWorkloads } } }
		}
		explicitEmpty: vulnerabilityFindings(workloadFilter: {teamScope: {teamSlugs: []}}) {
			affectedWorkloads { nodes { name } pageInfo { totalCount } }
		}
		noLabels: vulnerabilityFindings(workloadFilter: {labels: [{key: "absent"}]}) {
			affectedWorkloads { nodes { name } pageInfo { totalCount } }
		}
	}]], globalMetadata, findings))
	local empty = { nodes = {}, pageInfo = { totalCount = 0 } }
	t.check { data = {
		vulnerabilityFindings = {
			nodes = {}, pageInfo = { totalCount = 0 }, vulnerabilities = empty,
			facets = { packageTypes = {}, countsBySeverity = { critical = 0 } },
			summary = { findingCount = 0, affectedWorkloadCount = 0, coverage = { totalWorkloads = 0 } },
			affectedWorkloads = { nodes = {}, pageInfo = { totalCount = 0 }, facets = { vulnerabilityCoverage = { totalWorkloads = 0 } } },
		},
		explicitEmpty = { affectedWorkloads = empty }, noLabels = { affectedWorkloads = empty },
	} }
end)

Test.gql("Global explicit teams and membership are a union intersected with metadata", function(t)
	t.addHeader("x-user-email", member:email())
	t.query [[{
		vulnerabilityFindings(workloadFilter: {
			teamScope: {memberOfViewer: true, teamSlugs: ["v0s-team"]},
			environments: ["dev"], labels: [{key: "group", value: "core"}]
		}, filter: {vulnerabilityIdentifiers: ["CVE-shared"], onlyWithFix: true}) {
			affectedWorkloads { nodes { name } pageInfo { totalCount } }
		}
		unrestricted: vulnerabilityFindings(filter: {vulnerabilityIdentifiers: ["CVE-1"]}) {
			affectedWorkloads { nodes { name } pageInfo { totalCount } }
		}
	}]]
	t.check { data = {
		vulnerabilityFindings = { affectedWorkloads = { nodes = { { name = "alpha" }, { name = "zeta" } }, pageInfo = { totalCount = 2 } } },
		unrestricted = { affectedWorkloads = { nodes = { { name = "app" } }, pageInfo = { totalCount = 1 } } },
	} }
end)

local alphaKeys = [[
	{environment: "dev", workload: "alpha", imageName: "repo/dev/alpha", packageKey: "pkg:npm/first", vulnerabilityIdentifier: "CVE-shared"},
	{environment: "dev", workload: "alpha", imageName: "repo/dev/alpha", packageKey: "pkg:npm/second", vulnerabilityIdentifier: "CVE-shared"}
]]

Test.gql("Existing batch assessments are reflected in affected rows and full-selection counts", function(t)
	t.addHeader("x-user-email", member:email())
	t.query [[mutation {
		setVulnerabilitySuppressions(input: {team: "v0s-overview", items: [
			{key: {environment: "dev", workload: "alpha", imageName: "repo/dev/alpha", packageKey: "pkg:npm/first", vulnerabilityIdentifier: "CVE-shared"},
				state: ACCEPTED_RISK, reason: "reviewed"},
			{key: {environment: "dev", workload: "alpha", imageName: "repo/dev/alpha", packageKey: "pkg:npm/second", vulnerabilityIdentifier: "CVE-shared"},
				state: ACCEPTED_RISK, reason: "reviewed"}
		]}) { suppressions { packageKey state } }
	}]]
	t.check { data = { setVulnerabilitySuppressions = { suppressions = {
		{ packageKey = "pkg:npm/first",  state = "ACCEPTED_RISK" },
		{ packageKey = "pkg:npm/second", state = "ACCEPTED_RISK" },
	} } } }

	for _, mode in ipairs({ "ONLY", "EXCLUDE", "INCLUDE" }) do
		t.query(string.format([[{
			vulnerabilityFindings(first: 1, workloadFilter: %s,
				filter: {vulnerabilityIdentifiers: ["GHSA-shared"], packageType: "npm", suppressed: %s}) {
				summary { findingCount affectedWorkloadCount coverage { totalWorkloads } }
				affectedWorkloads { nodes { name } pageInfo { totalCount } }
			}
		}]], globalMetadata, mode))
		local names = { { name = "alpha" }, { name = "zeta" } }
		local count = 3
		if mode == "ONLY" then
			names, count = { { name = "alpha" } }, 2
		elseif mode == "EXCLUDE" then
			names, count = { { name = "zeta" } }, 1
		end
		t.check { data = { vulnerabilityFindings = {
			summary = { findingCount = count, affectedWorkloadCount = #names, coverage = { totalWorkloads = 8 } },
			affectedWorkloads = { nodes = names, pageInfo = { totalCount = #names } },
		} } }
	end

	t.query [[mutation {
		setVulnerabilitySuppressions(input: {team: "v0s-overview", items: [{
			key: {environment: "dev", workload: "alpha", imageName: "repo/dev/alpha", packageKey: "pkg:npm/first", vulnerabilityIdentifier: "CVE-shared"},
			state: FALSE_POSITIVE, reason: "Reviewed again"
		}]}) { suppressions { packageKey state } }
	}]]
	t.check { data = { setVulnerabilitySuppressions = { suppressions = {
		{ packageKey = "pkg:npm/first", state = "FALSE_POSITIVE" },
	} } } }

	t.query [[{
		team(slug: "v0s-overview") { environment(name: "dev") { workload(name: "alpha") {
			vulnerabilityFindings(filter: {vulnerabilityIdentifiers: ["CVE-shared"], suppressed: INCLUDE}) {
				nodes { packageKey image { name } suppression { state reason vulnerability { identifier } } }
			}
		} } }
	}]]
	t.check { data = { team = { environment = { workload = { vulnerabilityFindings = { nodes = {
		{ packageKey = "pkg:npm/first", image = { name = "repo/dev/alpha" },
			suppression = { state = "FALSE_POSITIVE", reason = "Reviewed again", vulnerability = { identifier = "CVE-shared" } } },
		{ packageKey = "pkg:npm/second", image = { name = "repo/dev/alpha" },
			suppression = { state = "ACCEPTED_RISK", reason = "reviewed", vulnerability = { identifier = "CVE-shared" } } },
	} } } } } } }

	t.query(string.format([[mutation {
		deleteVulnerabilitySuppressions(input: {team: "v0s-overview", keys: [%s]}) { deleted { packageKey } }
	}]], alphaKeys))
	t.check { data = { deleteVulnerabilitySuppressions = { deleted = {
		{ packageKey = "pkg:npm/first" }, { packageKey = "pkg:npm/second" },
	} } } }
	t.query(string.format([[{
		vulnerabilityFindings(workloadFilter: %s, filter: {vulnerabilityIdentifiers: ["CVE-shared"]}) {
			affectedWorkloads { nodes { name } pageInfo { totalCount } }
		}
	}]], globalMetadata))
	t.check { data = { vulnerabilityFindings = { affectedWorkloads = {
		nodes = { { name = "alpha" }, { name = "zeta" } }, pageInfo = { totalCount = 2 },
	} } } }
end)
