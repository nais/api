Config.V0s = true
local member = User.new()
local outsider = User.new()
local team = Team.new("v0s-team", "Vulnerability assessments", "#v0s")
team:addMember(member)
Helper.readK8sResources("k8s_resources/v0s")

local app =
[[{environment: "dev", workload: "app", imageName: "repo/app", packageKey: "pkg:npm/example", vulnerabilityIdentifier: "CVE-1"}]]
local stale =
[[{environment: "dev", workload: "stale", imageName: "repo/stale", packageKey: "pkg:npm/other", vulnerabilityIdentifier: "CVE-2"}]]
local function set(reason)
	return string.format([[mutation { setVulnerabilitySuppressions(input: {team: "v0s-team", items: [
		{key: %s, state: ACCEPTED_RISK, reason: "assessed"},
		{key: %s, state: FALSE_POSITIVE, reason: "%s"}
	]}) { suppressions { id workload { name } vulnerability { identifier } updatedBy } } }]], app, stale, reason)
end
local delete = string.format([[mutation {
	deleteVulnerabilitySuppressions(input: {team: "v0s-team", keys: [%s, %s]}) { deleted { workloadName } }
}]], app, stale)

Test.gql("Assessment writes reject callers without owning-team authorization", function(t)
	t.addHeader("x-user-email", outsider:email())
	for _, query in ipairs({ set("assessed"), delete }) do
		t.query(query)
		t.check { data = Null, errors = { { locations = NotNull(), message = Contains('you need the "vulnerability:update" authorization'), path = NotNull() } } }
	end
end)

Test.gql("Authorized assessment batches are validated and recorded through the public API", function(t)
	t.addHeader("x-user-email", member:email())
	t.query(set(""))
	t.check { data = Null, errors = { { locations = NotNull(), message = "reason cannot be empty", path = { "setVulnerabilitySuppressions" } } } }
	local list =
	[[{ team(slug: "v0s-team") { vulnerabilitySuppressions { nodes { state } pageInfo { totalCount } } } }]]
	t.query(list)
	t.check { data = { team = { vulnerabilitySuppressions = { nodes = {}, pageInfo = { totalCount = 0 } } } } }

	t.query(set("assessed"))
	t.check { data = { setVulnerabilitySuppressions = { suppressions = {
		{ id = Save("assessment_id"), workload = { name = "app" },   vulnerability = { identifier = "CVE-1" }, updatedBy = member:email() },
		{ id = NotNull(),             workload = { name = "stale" }, vulnerability = { identifier = "CVE-2" }, updatedBy = member:email() },
	} } } }
	t.query [[{
		vulnerabilitySuppressions(first: 1) {
			nodes { workload { name } }
			pageInfo { totalCount hasNextPage hasPreviousPage endCursor }
		}
	}]]
	t.check { data = { vulnerabilitySuppressions = {
		nodes = { { workload = { name = "app" } } },
		pageInfo = { totalCount = 2, hasNextPage = true, hasPreviousPage = false, endCursor = Save("suppression_first_cursor") },
	} } }
	t.query(string.format([[{
		vulnerabilitySuppressions(first: 1, after: "%s") {
			nodes { workload { name } }
			pageInfo { totalCount hasNextPage hasPreviousPage startCursor }
		}
	}]], State.suppression_first_cursor))
	t.check { data = { vulnerabilitySuppressions = {
		nodes = { { workload = { name = "stale" } } },
		pageInfo = { totalCount = 2, hasNextPage = false, hasPreviousPage = true, startCursor = Save("suppression_second_cursor") },
	} } }
	t.query(string.format([[{
		vulnerabilitySuppressions(last: 20, before: "%s") {
			nodes { workload { name } }
			pageInfo { totalCount hasNextPage hasPreviousPage endCursor pageStart pageEnd }
		}
	}]], State.suppression_second_cursor))
	t.check { data = { vulnerabilitySuppressions = {
		nodes = { { workload = { name = "app" } } },
		pageInfo = { totalCount = 2, hasNextPage = true, hasPreviousPage = false, endCursor = State.suppression_first_cursor, pageStart = 1, pageEnd = 1 },
	} } }
	t.query [[mutation {
		deleteApplication(input: {teamSlug: "v0s-team", environmentName: "dev", name: "stale"}) { success }
	}]]
	t.check { data = { deleteApplication = { success = true } } }
	t.query(string.format([[{
		node(id: "%s") { ... on VulnerabilitySuppression { state reason workload { name teamEnvironment { team { slug } } } } }
		afterApp: vulnerabilitySuppressions(first: 1, after: "%s") {
			nodes { id }
			pageInfo { totalCount hasNextPage hasPreviousPage startCursor endCursor }
		}
		beforeRemoved: vulnerabilitySuppressions(last: 1, before: "%s") {
			nodes { workload { name } }
			pageInfo { totalCount hasNextPage hasPreviousPage }
		}
		team(slug: "v0s-team") { environment(name: "dev") { vulnerabilitySuppressions {
			nodes { workload { name } vulnerability { identifier } state }
			pageInfo { totalCount }
		} } }
	}]], State.assessment_id, State.suppression_first_cursor, State.suppression_second_cursor))
	t.check { data = {
		node = { state = "ACCEPTED_RISK", reason = "assessed", workload = { name = "app", teamEnvironment = { team = { slug = "v0s-team" } } } },
		afterApp = { nodes = {}, pageInfo = {
			totalCount = 1, hasNextPage = false, hasPreviousPage = true, startCursor = Null, endCursor = Null,
		} },
		beforeRemoved = { nodes = { { workload = { name = "app" } } },
			pageInfo = { totalCount = 1, hasNextPage = false, hasPreviousPage = false },
		},
		team = { environment = { vulnerabilitySuppressions = {
			nodes = {
				{ workload = { name = "app" }, vulnerability = { identifier = "CVE-1" }, state = "ACCEPTED_RISK" },
			},
			pageInfo = { totalCount = 1 },
		} } },
	} }
	t.query(delete)
	t.check { data = { deleteVulnerabilitySuppressions = { deleted = { { workloadName = "app" }, { workloadName = "stale" } } } } }
	t.query [[{
		vulnerabilitySuppressions { nodes { id } }
		team(slug: "v0s-team") {
			updated: activityLog(filter: {activityTypes: [VULNERABILITY_SUPPRESSION_UPDATED]}) { pageInfo { totalCount } }
			deleted: activityLog(filter: {activityTypes: [VULNERABILITY_SUPPRESSION_DELETED]}) {
				nodes { actor environmentName resourceName ... on VulnerabilitySuppressionActivityLogEntry {
					data { assessment { workloadName reason } }
				} }
			}
		}
	}]]
	local function entry(i)
		return {
			actor = member:email(),
			environmentName = "dev",
			resourceName = Save("resource" .. i),
			data = { assessment = { workloadName = Save("workload" .. i), reason = "assessed" } },
		}
	end
	t.check { data = { vulnerabilitySuppressions = { nodes = {} }, team = {
		updated = { pageInfo = { totalCount = 2 } },
		deleted = { nodes = { entry(1), entry(2) } },
	} } }
	assert(State.resource1 == State.workload1 and State.resource2 == State.workload2)
	assert((State.workload1 == "app" and State.workload2 == "stale") or
		(State.workload1 == "stale" and State.workload2 == "app"))
end)

Test.sql("Reject assessment activity inserts for the test team", function()
	Helper.SQLExec([[
		ALTER TABLE activity_log_entries ADD CONSTRAINT reject_v0s_test_activity
		CHECK (team_slug <> 'v0s-team' OR resource_type <> 'VULNERABILITY_SUPPRESSION') NOT VALID
	]])
end)

Test.gql("Set and delete succeed even when writing activity fails", function(t)
	t.addHeader("x-user-email", member:email())
	t.query(string.format([[mutation {
		setVulnerabilitySuppressions(input: {team: "v0s-team", items: [{
			key: %s, state: ACCEPTED_RISK, reason: "assessed"
		}]}) { suppressions { reason } }
	}]], app))
	t.check { data = { setVulnerabilitySuppressions = { suppressions = { { reason = "assessed" } } } } }

	t.query(delete)
	t.check { data = { deleteVulnerabilitySuppressions = { deleted = { { workloadName = "app" } } } } }

	t.query [[{
		team(slug: "v0s-team") {
			vulnerabilitySuppressions { pageInfo { totalCount } }
			updated: activityLog(filter: {activityTypes: [VULNERABILITY_SUPPRESSION_UPDATED]}) { pageInfo { totalCount } }
			deleted: activityLog(filter: {activityTypes: [VULNERABILITY_SUPPRESSION_DELETED]}) { pageInfo { totalCount } }
		}
	}]]
	t.check { data = { team = {
		vulnerabilitySuppressions = { pageInfo = { totalCount = 0 } },
		updated = { pageInfo = { totalCount = 2 } },
		deleted = { pageInfo = { totalCount = 2 } },
	} } }
end)

Test.sql("Restore assessment activity inserts", function()
	Helper.SQLExec("ALTER TABLE activity_log_entries DROP CONSTRAINT reject_v0s_test_activity")
end)
