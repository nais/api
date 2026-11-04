local member = User.new()
local admin = User.new()
admin:admin(true)
for i = 1, 25 do
	local slug = string.format("scope-%02d", i)
	local team = Team.new(slug, "Scope testing", "#scope")
	team:addMember(member)
	Helper.SQLExec(string.format([[
		INSERT INTO issues (id, issue_type, resource_name, resource_type, team, env, severity)
		VALUES ('00000000-0000-0000-0000-%012d', 'NO_RUNNING_INSTANCES', 'app', 'APPLICATION', '%s', 'dev', 'CRITICAL');
		INSERT INTO activity_log_entries (id, created_at, actor, action, resource_type, resource_name, team_slug, environment, data)
		VALUES ('00000000-0000-0000-0000-%012d', '2026-01-01T00:00:00Z', 'deployer', 'DEPLOYMENT', 'APP', 'app', '%s', 'dev', '{}');
	]], i, slug, i, slug))
end
Team.new("outside", "Not a membership", "#outside")
Helper.SQLExec([[
	INSERT INTO issues (issue_type, resource_name, resource_type, team, env, severity)
	VALUES
		('NO_RUNNING_INSTANCES', 'outside', 'APPLICATION', 'outside', 'dev', 'CRITICAL'),
		('NO_RUNNING_INSTANCES', 'prod-app', 'APPLICATION', 'scope-01', 'prod', 'WARNING');
	INSERT INTO issues (issue_type, resource_name, resource_type, team, env, severity)
	SELECT 'NO_RUNNING_INSTANCES', 'orphan-' || i, 'APPLICATION', 'unknown', 'dev', 'CRITICAL'
	FROM generate_series(1, 30) AS i;
	INSERT INTO activity_log_entries (actor, action, resource_type, resource_name, team_slug, environment, data)
	VALUES
		('deployer', 'DEPLOYMENT', 'APP', 'outside', 'outside', 'dev', '{}'),
		('deployer', 'DEPLOYMENT', 'APP', 'tenant', NULL, 'dev', '{}');
]])

Test.gql("Issue team facets exclude nonexistent teams before limiting", function(t)
	t.addHeader("x-user-email", member:email())
	t.query [[
		{
			issues(first: 1, filter: {severity: CRITICAL, environments: ["dev"]}) {
				pageInfo { totalCount }
				facets {
					teams { team { slug } count }
					severities { severity count }
					resourceTypes { resourceType count }
					environments { value count }
					issueTypes { issueType count }
				}
			}
			unknown: issues(filter: {teamScope: {teamSlugs: ["unknown"]}}) {
				pageInfo { totalCount }
				facets { teams { team { slug } count } severities { severity count } }
			}
		}
	]]
	local teams = { { team = { slug = "outside" }, count = 1 } }
	for i = 1, 9 do
		table.insert(teams, { team = { slug = string.format("scope-%02d", i) }, count = 1 })
	end
	t.check { data = {
		issues = {
			pageInfo = { totalCount = 56 },
			facets = {
				teams = teams,
				severities = { { severity = "CRITICAL", count = 56 }, { severity = "WARNING", count = 0 } },
				resourceTypes = { { resourceType = "APPLICATION", count = 56 } },
				environments = { { value = "dev", count = 56 }, { value = "prod", count = 0 } },
				issueTypes = { { issueType = "NO_RUNNING_INSTANCES", count = 56 } },
			},
		},
		unknown = {
			pageInfo = { totalCount = 30 },
			facets = { teams = {}, severities = { { severity = "CRITICAL", count = 30 } } },
		},
	} }
end)

Test.gql("Team discovery scopes precede ordering and pagination", function(t)
	t.addHeader("x-user-email", member:email())
	t.query [[
		{
			teams(first: 1, orderBy: {field: SLUG, direction: DESC}, filter: {teamScope: {memberOfViewer: true}}) {
				nodes { slug }
				pageInfo { totalCount hasNextPage endCursor }
			}
			selected: teams(first: 30, filter: {teamScope: {teamSlugs: ["scope-25", "outside"], memberOfViewer: true}, hasWorkloads: false}) {
				nodes { slug }
				pageInfo { totalCount }
			}
			withWorkloads: teams(filter: {teamScope: {teamSlugs: ["scope-25"]}, hasWorkloads: true}) {
				nodes { slug }
				pageInfo { totalCount }
			}
		}
	]]
	local selectedTeams = { { slug = "outside" } }
	for i = 1, 25 do
		table.insert(selectedTeams, { slug = string.format("scope-%02d", i) })
	end
	t.check { data = {
		teams = {
			nodes = { { slug = "scope-25" } },
			pageInfo = { totalCount = 25, hasNextPage = true, endCursor = Save("teamsEnd") },
		},
		selected = { nodes = selectedTeams, pageInfo = { totalCount = 26 } },
		withWorkloads = { nodes = {}, pageInfo = { totalCount = 0 } },
	} }
	t.query(string.format([[
		{
			teams(first: 30, after: "%s", orderBy: {field: SLUG, direction: DESC}, filter: {teamScope: {memberOfViewer: true}}) {
				nodes { slug }
				pageInfo { totalCount hasNextPage endCursor }
			}
		}
	]], State.teamsEnd))
	local remaining = {}
	for i = 24, 1, -1 do
		table.insert(remaining, { slug = string.format("scope-%02d", i) })
	end
	t.check { data = {
		teams = {
			nodes = remaining,
			pageInfo = { totalCount = 25, hasNextPage = false, endCursor = Save("teamsLast") },
		},
	} }
	t.query(string.format([[
		{
			teams(first: 1, after: "%s", filter: {teamScope: {memberOfViewer: true}}) {
				nodes { slug }
				pageInfo { totalCount }
			}
		}
	]], State.teamsLast))
	t.check { data = { teams = { nodes = {}, pageInfo = { totalCount = 25 } } } }
end)

Test.gql("Scopes include memberships beyond the first page", function(t)
	t.addHeader("x-user-email", member:email())
	t.query [[
		{
			me { ... on User { teams(first: 1) { pageInfo { totalCount } } } }
			issues(first: 1, filter: {teamScope: {memberOfViewer: true}, severity: CRITICAL, environments: ["dev"]}) {
				nodes { severity }
				pageInfo { totalCount hasNextPage endCursor }
				facets { teams { team { slug } count } }
			}
			activityLog(first: 1, filter: {teamScope: {memberOfViewer: true}, activityTypes: [DEPLOYMENT]}) {
				nodes { teamSlug }
				pageInfo { totalCount hasNextPage endCursor }
				facets { resourceTypes { resourceType count } }
			}
		}
	]]
	local teams = {}
	for i = 1, 10 do
		table.insert(teams, { team = { slug = string.format("scope-%02d", i) }, count = 1 })
	end
	t.check { data = {
		me = { teams = { pageInfo = { totalCount = 25 } } },
		issues = {
			nodes = { { severity = "CRITICAL" } },
			pageInfo = { totalCount = 25, hasNextPage = true, endCursor = Save("issuesEnd") },
			facets = { teams = teams },
		},
		activityLog = {
			nodes = { { teamSlug = "scope-25" } },
			pageInfo = { totalCount = 25, hasNextPage = true, endCursor = Save("activityEnd") },
			facets = { resourceTypes = { { resourceType = "APP", count = 25 } } },
		},
	} }
	t.query(string.format([[
		{
			issues(first: 30, after: "%s", filter: {teamScope: {memberOfViewer: true}, severity: CRITICAL}) {
				pageInfo { totalCount endCursor }
			}
			activityLog(first: 30, after: "%s", filter: {teamScope: {memberOfViewer: true}, activityTypes: [DEPLOYMENT]}) {
				nodes { teamSlug }
				pageInfo { totalCount endCursor }
			}
		}
	]], State.issuesEnd, State.activityEnd))
	local remaining = {}
	for i = 24, 1, -1 do
		table.insert(remaining, { teamSlug = string.format("scope-%02d", i) })
	end
	t.check { data = {
		issues = { pageInfo = { totalCount = 25, endCursor = Save("issuesLast") } },
		activityLog = {
			nodes = remaining,
			pageInfo = { totalCount = 25, endCursor = Save("activityLast") },
		},
	} }
	t.query(string.format([[
		{
			issues(first: 1, after: "%s", filter: {teamScope: {memberOfViewer: true}, severity: CRITICAL}) {
				nodes { id }
				pageInfo { totalCount }
			}
			activityLog(first: 1, after: "%s", filter: {teamScope: {memberOfViewer: true}, activityTypes: [DEPLOYMENT]}) {
				nodes { id }
				pageInfo { totalCount }
			}
		}
	]], State.issuesLast, State.activityLast))
	t.check { data = {
		issues = { nodes = {}, pageInfo = { totalCount = 25 } },
		activityLog = { nodes = {}, pageInfo = { totalCount = 25 } },
	} }
end)

Test.gql("Tenant scopes and team filters are independent", function(t)
	t.addHeader("x-user-email", member:email())
	t.query [[
		{
			issues(filter: {teamScope: {teamSlugs: ["scope-25", "outside"], memberOfViewer: true}}) {
				pageInfo { totalCount }
				facets { teams { team { slug } count } }
			}
			activityLog(first: 30, filter: {teamScope: {teamSlugs: ["scope-25", "outside"], memberOfViewer: true}}) {
				nodes { teamSlug }
				pageInfo { totalCount }
			}
			team(slug: "scope-01") {
				issues(filter: {severity: WARNING, environments: ["prod"]}) {
					pageInfo { totalCount }
					facets { teams { count } }
				}
				activityLog(filter: {activityTypes: [DEPLOYMENT], environments: ["dev"]}) {
					pageInfo { totalCount }
					facets { resourceTypes { count } }
				}
			}
		}
	]]
	local teamFacets = {
		{ team = { slug = "scope-01" }, count = 2 },
		{ team = { slug = "outside" },  count = 1 },
	}
	for i = 2, 9 do
		table.insert(teamFacets, { team = { slug = string.format("scope-%02d", i) }, count = 1 })
	end
	local activityNodes = { { teamSlug = "outside" } }
	for i = 25, 1, -1 do
		table.insert(activityNodes, { teamSlug = string.format("scope-%02d", i) })
	end
	t.check { data = {
		issues = {
			pageInfo = { totalCount = 27 },
			facets = { teams = teamFacets },
		},
		activityLog = {
			nodes = activityNodes,
			pageInfo = { totalCount = 26 },
		},
		team = {
			issues = { pageInfo = { totalCount = 1 }, facets = { teams = { { count = 1 } } } },
			activityLog = { pageInfo = { totalCount = 1 }, facets = { resourceTypes = { { count = 1 } } } },
		},
	} }
end)

Test.gql("Empty explicit selection still includes requested memberships", function(t)
	t.addHeader("x-user-email", member:email())
	t.query [[
		{
			teams(filter: {teamScope: {teamSlugs: [], memberOfViewer: true}}) { pageInfo { totalCount } }
			issues(filter: {teamScope: {teamSlugs: [], memberOfViewer: true}}) { pageInfo { totalCount } }
			activityLog(filter: {teamScope: {teamSlugs: [], memberOfViewer: true}}) { pageInfo { totalCount } }
		}
	]]
	t.check { data = {
		teams = { pageInfo = { totalCount = 25 } },
		issues = { pageInfo = { totalCount = 26 } },
		activityLog = { pageInfo = { totalCount = 25 } },
	} }
end)

for _, selection in ipairs({ "teamSlugs: []", "teamSlugs: [], memberOfViewer: false" }) do
	Test.gql("Explicit empty scope: " .. selection, function(t)
		t.addHeader("x-user-email", member:email())
		t.query(string.format([[
			{
				teams(filter: {teamScope: {%s}}) {
					nodes { slug }
					pageInfo { totalCount }
				}
				issues(filter: {teamScope: {%s}}) {
					nodes { id }
					pageInfo { totalCount }
					facets { teams { count } }
				}
				activityLog(filter: {teamScope: {%s}}) {
					nodes { id }
					pageInfo { totalCount }
					facets { resourceTypes { count } }
				}
			}
		]], selection, selection, selection))
		t.check { data = {
			teams = { nodes = {}, pageInfo = { totalCount = 0 } },
			issues = { nodes = {}, pageInfo = { totalCount = 0 }, facets = { teams = {} } },
			activityLog = { nodes = {}, pageInfo = { totalCount = 0 }, facets = { resourceTypes = {} } },
		} }
	end)
end

Test.gql("Admin access does not imply membership", function(t)
	t.addHeader("x-user-email", admin:email())
	t.query [[
		{
			teams(filter: {teamScope: {memberOfViewer: true}}) { pageInfo { totalCount } }
			issues(filter: {teamScope: {memberOfViewer: true}}) { pageInfo { totalCount } }
			activityLog(filter: {teamScope: {memberOfViewer: true}}) { pageInfo { totalCount } }
			explicit: issues(filter: {teamScope: {teamSlugs: ["outside"]}}) { pageInfo { totalCount } }
		}
	]]
	t.check { data = {
		teams = { pageInfo = { totalCount = 0 } },
		issues = { pageInfo = { totalCount = 0 } },
		activityLog = { pageInfo = { totalCount = 0 } },
		explicit = { pageInfo = { totalCount = 1 } },
	} }
end)
