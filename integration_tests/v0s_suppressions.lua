Config.V0s = true
Config.V0sSuppressionsEnabled = true
local member = User.new()
local outsider = User.new()
local team = Team.new("v0s-team", "Vulnerability assessments", "#v0s")
team:addMember(member)

local app =
[[{environment: "dev", workload: "app", imageName: "repo/app", packageKey: "pkg:npm/example", vulnerabilityIdentifier: "CVE-1"}]]
local removed =
[[{environment: "dev", workload: "removed", imageName: "repo/removed", packageKey: "pkg:npm/other", vulnerabilityIdentifier: "CVE-2"}]]
local function set(reason)
	return string.format([[mutation { setVulnerabilitySuppressions(input: {team: "v0s-team", items: [
		{key: %s, state: ACCEPTED_RISK, reason: "assessed"},
		{key: %s, state: FALSE_POSITIVE, reason: "%s"}
	]}) { suppressions { id workload updatedBy } } }]], app, removed, reason)
end
local delete = string.format([[mutation {
	deleteVulnerabilitySuppressions(input: {team: "v0s-team", keys: [%s, %s]}) { deleted { workload } }
}]], app, removed)

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
	[[{ vulnerabilitySuppressions(filter: {scope: {team: "v0s-team"}}) { nodes { workload state } pageInfo { totalCount } } }]]
	t.query(list)
	t.check { data = { vulnerabilitySuppressions = { nodes = {}, pageInfo = { totalCount = 0 } } } }

	t.query(set("assessed"))
	t.check { data = { setVulnerabilitySuppressions = { suppressions = {
		{ id = Save("assessment_id"), workload = "app",     updatedBy = member:email() },
		{ id = NotNull(),             workload = "removed", updatedBy = member:email() },
	} } } }
	t.query(string.format([[{
		node(id: "%s") { ... on VulnerabilitySuppression { workload state reason } }
		vulnerabilitySuppressions(filter: {scope: {team: "v0s-team"}}) { nodes { workload state } pageInfo { totalCount } }
	}]], State.assessment_id))
	t.check { data = {
		node = { workload = "app", state = "ACCEPTED_RISK", reason = "assessed" },
		vulnerabilitySuppressions = { nodes = { { workload = "app", state = "ACCEPTED_RISK" }, { workload = "removed", state = "FALSE_POSITIVE" } }, pageInfo = { totalCount = 2 } },
	} }
	t.query(delete)
	t.check { data = { deleteVulnerabilitySuppressions = { deleted = { { workload = "app" }, { workload = "removed" } } } } }
	t.query [[{
		vulnerabilitySuppressions(filter: {scope: {team: "v0s-team"}}) { nodes { id } }
		team(slug: "v0s-team") {
			updated: activityLog(filter: {activityTypes: [VULNERABILITY_SUPPRESSION_UPDATED]}) { pageInfo { totalCount } }
			deleted: activityLog(filter: {activityTypes: [VULNERABILITY_SUPPRESSION_DELETED]}) {
				nodes { actor environmentName resourceName ... on VulnerabilitySuppressionActivityLogEntry {
					data { assessment { workload reason } }
				} }
			}
		}
	}]]
	local function entry(i)
		return {
			actor = member:email(),
			environmentName = "dev",
			resourceName = Save("resource" .. i),
			data = { assessment = { workload = Save("workload" .. i), reason = "assessed" } },
		}
	end
	t.check { data = { vulnerabilitySuppressions = { nodes = {} }, team = {
		updated = { pageInfo = { totalCount = 2 } },
		deleted = { nodes = { entry(1), entry(2) } },
	} } }
	assert(State.resource1 == State.workload1 and State.resource2 == State.workload2)
	assert((State.workload1 == "app" and State.workload2 == "removed") or
		(State.workload1 == "removed" and State.workload2 == "app"))
end)
