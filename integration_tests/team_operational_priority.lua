local user = User.new()
for _, slug in ipairs({ "high-a", "high-b", "elevated-a", "elevated-b", "monitor", "none", "no-sbom" }) do
	Team.new(slug, "purpose", "#channel")
end
for index = 1, 25 do
	Team.new(string.format("empty-%02d", index), "purpose", "#channel")
end
Helper.readK8sResources("k8s_resources/team_operational_priority")

local function nodes(slugs)
	local result = {}
	for _, slug in ipairs(slugs) do
		table.insert(result, { slug = slug })
	end
	return result
end

local function connection(slugs, total, previous, following, key)
	return {
		nodes = nodes(slugs),
		pageInfo = {
			totalCount = total,
			hasPreviousPage = previous,
			hasNextPage = following,
			startCursor = Save(key .. "Start"),
			endCursor = Save(key .. "End"),
		},
	}
end

local selection = [[
	nodes { slug }
	pageInfo { totalCount hasPreviousPage hasNextPage startCursor endCursor }
]]

Test.gql("Operational groups have independent cursors and exclude lower priorities", function(t)
	t.addHeader("x-user-email", user:email())
	local fetch = function(highAfter, elevatedAfter)
		t.query(string.format([[
			{
				high: teams(first: 1, after: "%s", filter: {hasWorkloads: true, vulnerabilityPriorityGroup: HIGH}) { %s }
				elevated: teams(first: 1, after: "%s", filter: {hasWorkloads: true, vulnerabilityPriorityGroup: ELEVATED}) { %s }
				monitor: teams(first: 20, filter: {hasWorkloads: true, vulnerabilityPriorityGroup: MONITOR}) { %s }
				none: teams(first: 20, filter: {hasWorkloads: true, vulnerabilityPriorityGroup: NONE}) { %s }
			}
		]], highAfter, selection, elevatedAfter, selection, selection, selection))
	end
	fetch("", "")
	t.check { data = {
		high = connection({ "high-a" }, 2, false, true, "high"),
		elevated = connection({ "elevated-a" }, 2, false, true, "elevated"),
		monitor = connection({ "monitor" }, 1, false, false, "monitor"),
		none = connection({ "no-sbom", "none" }, 2, false, false, "none"),
	} }
	local highEnd = State.highEnd
	local elevatedEnd = State.elevatedEnd
	fetch(highEnd, "")
	t.check { data = {
		high = connection({ "high-b" }, 2, true, false, "highNext"),
		elevated = connection({ "elevated-a" }, 2, false, true, "elevated"),
		monitor = connection({ "monitor" }, 1, false, false, "monitor"),
		none = connection({ "no-sbom", "none" }, 2, false, false, "none"),
	} }
	fetch(highEnd, elevatedEnd)
	t.check { data = {
		high = connection({ "high-b" }, 2, true, false, "highNext"),
		elevated = connection({ "elevated-b" }, 2, true, false, "elevatedNext"),
		monitor = connection({ "monitor" }, 1, false, false, "monitor"),
		none = connection({ "no-sbom", "none" }, 2, false, false, "none"),
	} }
	t.query(string.format([[
		{ teams(last: 1, before: "%s", filter: {hasWorkloads: true, vulnerabilityPriorityGroup: HIGH}) { %s } }
	]], State.highNextStart, selection))
	t.check { data = { teams = connection({ "high-a" }, 2, false, true, "previous") } }
end)

Test.gql("Group totals cover more than twenty teams across next and previous pages", function(t)
	t.addHeader("x-user-email", user:email())
	local firstSlugs = {}
	local nextSlugs = {}
	for index = 1, 25 do
		table.insert(index <= 20 and firstSlugs or nextSlugs, string.format("empty-%02d", index))
	end
	local fetch = function(arguments)
		t.query(string.format([[
			{ teams(%s, filter: {hasWorkloads: false, vulnerabilityPriorityGroup: NONE}) { %s } }
		]], arguments, selection))
	end
	fetch("first: 20")
	t.check { data = { teams = connection(firstSlugs, 25, false, true, "emptyFirst") } }
	fetch(string.format('first: 20, after: "%s"', State.emptyFirstEnd))
	t.check { data = { teams = connection(nextSlugs, 25, true, false, "emptyNext") } }
	fetch(string.format('last: 20, before: "%s"', State.emptyNextStart))
	t.check { data = { teams = connection(firstSlugs, 25, false, true, "emptyPrevious") } }
	fetch(string.format('first: 20, after: "%s"', State.emptyNextEnd))
	t.check { data = { teams = {
		nodes = {},
		pageInfo = { totalCount = 25, hasPreviousPage = true, hasNextPage = false, startCursor = Null, endCursor = Null },
	} } }
end)

Test.gql("Explicit ordering and existing filters are preserved", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		{
			high: teams(first: 20, orderBy: {field: SLUG, direction: DESC}, filter: {vulnerabilityPriorityGroup: HIGH}) { %s }
			none: teams(first: 1, filter: {vulnerabilityPriorityGroup: NONE}) { %s }
			workloads: teams(first: 20, filter: {hasWorkloads: true}) { %s }
			nullFilter: teams(first: 20, filter: {hasWorkloads: true, vulnerabilityPriorityGroup: null}) { %s }
			unfiltered: teams(first: 1) { %s }
		}
	]], selection, selection, selection, selection, selection))
	local workloads = { "elevated-a", "elevated-b", "high-a", "high-b", "monitor", "no-sbom", "none" }
	t.check { data = {
		high = connection({ "high-b", "high-a" }, 2, false, false, "ordered"),
		none = connection({ "empty-01" }, 27, false, true, "allNone"),
		workloads = connection(workloads, 7, false, false, "workloads"),
		nullFilter = connection(workloads, 7, false, false, "nullFilter"),
		unfiltered = connection({ "elevated-a" }, 32, false, true, "unfiltered"),
	} }
end)

Test.gql("High findings do not require KEV and lower groups never include High", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		{
			teams(first: 20, filter: {hasWorkloads: true, vulnerabilityPriorityGroup: HIGH}) {
				nodes {
					slug
					vulnerabilitySummary { countsByPriority { highRisk elevatedRisk monitor knownExploited } }
				}
			}
		}
	]]
	local summary = { countsByPriority = { highRisk = 3, elevatedRisk = 100, monitor = 100, knownExploited = 0 } }
	t.check { data = { teams = { nodes = {
		{ slug = "high-a", vulnerabilitySummary = summary },
		{ slug = "high-b", vulnerabilitySummary = summary },
	} } } }
end)
