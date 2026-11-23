Config.V0s = true
local member = User.new()
local outsider = User.new()
local team = Team.new("v0s-team", "Vulnerability assessments", "#v0s")
team:addMember(member)
Helper.readK8sResources("k8s_resources/v0s")

local app =
[[{environment: "dev", workload: "app", imageName: "repo/app", packageKey: "pkg:npm/example", vulnerabilityIdentifier: "CVE-1"}]]
local removed =
[[{environment: "dev", workload: "removed", imageName: "repo/removed", packageKey: "pkg:npm/other", vulnerabilityIdentifier: "CVE-2"}]]
local function set(reason)
	return string.format([[mutation { setVulnerabilitySuppressions(input: {team: "v0s-team", items: [
		{key: %s, state: ACCEPTED_RISK, reason: "assessed"},
		{key: %s, state: FALSE_POSITIVE, reason: "%s"}
	]}) { suppressions { id workloadName updatedBy } } }]], app, removed, reason)
end
local delete = string.format([[mutation {
	deleteVulnerabilitySuppressions(input: {team: "v0s-team", keys: [%s, %s]}) { deleted { workloadName } }
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
	[[{ team(slug: "v0s-team") { vulnerabilitySuppressions { nodes { workloadName state } pageInfo { totalCount } } } }]]
	t.query(list)
	t.check { data = { team = { vulnerabilitySuppressions = { nodes = {}, pageInfo = { totalCount = 0 } } } } }

	t.query(set("assessed"))
	t.check { data = { setVulnerabilitySuppressions = { suppressions = {
		{ id = Save("assessment_id"), workloadName = "app",     updatedBy = member:email() },
		{ id = NotNull(),             workloadName = "removed", updatedBy = member:email() },
	} } } }
	t.query(string.format([[{
		node(id: "%s") { ... on VulnerabilitySuppression { workloadName state reason teamEnvironment { team { slug } } } }
		team(slug: "v0s-team") { environment(name: "dev") { vulnerabilitySuppressions {
			nodes { workloadName workload { name } vulnerability { identifier } state }
			pageInfo { totalCount }
		} } }
	}]], State.assessment_id))
	t.check { data = {
		node = { workloadName = "app", state = "ACCEPTED_RISK", reason = "assessed", teamEnvironment = { team = { slug = "v0s-team" } } },
		team = { environment = { vulnerabilitySuppressions = {
			nodes = {
				{ workloadName = "app",     workload = { name = "app" }, vulnerability = { identifier = "CVE-1" }, state = "ACCEPTED_RISK" },
				{ workloadName = "removed", workload = Null,             vulnerability = { identifier = "CVE-2" }, state = "FALSE_POSITIVE" },
			},
			pageInfo = { totalCount = 2 },
		} } },
	} }
	t.query(delete)
	t.check { data = { deleteVulnerabilitySuppressions = { deleted = { { workloadName = "app" }, { workloadName = "removed" } } } } }
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
	assert((State.workload1 == "app" and State.workload2 == "removed") or
		(State.workload1 == "removed" and State.workload2 == "app"))
end)
