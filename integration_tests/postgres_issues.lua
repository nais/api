local user = User.new("user", "user@usersen.com")
local team = Team.new("someteamname", "purpose", "#slack_channel")
team:addMember(user)
Team.new("postgres-other-team", "Isolation", "#postgres")
Helper.readK8sResources("k8s_resources/postgres_branches")
local checker = IssueChecker.new()
checker:runChecks()

Test.gql("Degraded Postgres produces a genuine Postgres issue", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{
		team(slug: "someteamname") {
			issues(filter: { issueType: POSTGRES, resourceType: POSTGRES }) {
				pageInfo { totalCount }
				nodes {
					__typename id severity message
					teamEnvironment { name }
					... on PostgresIssue {
						postgres { name }
					}
				}
			}
		}
	}]]
	t.check { data = { team = { issues = {
		pageInfo = { totalCount = 1 },
		nodes = { {
			__typename = "PostgresIssue",
			id = Save("postgres_issue_id"),
			severity = "CRITICAL",
			postgres = { name = "broken" },
			message = 'Postgres "broken" has reported a failure.',
			teamEnvironment = { name = "dev" },
		} },
	} } } }
end)

Test.gql("Postgres issue is a retrievable Node", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[{
		node(id: "%s") {
			__typename
			... on PostgresIssue { id postgres { name } }
		}
	}]], State.postgres_issue_id))
	t.check { data = { node = {
		__typename = "PostgresIssue", id = State.postgres_issue_id, postgres = { name = "broken" },
	} } }
end)

Test.gql("Postgres issues are scoped to the logical database", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{
		team(slug: "someteamname") {
			environment(name: "dev") {
				postgres(name: "broken") {
					issues(filter: { severity: CRITICAL }) {
						pageInfo { totalCount } nodes { __typename }
						facets { resourceTypes { resourceType count } issueTypes { issueType count } }
					}
				}
				other: postgres(name: "another-db") {
					issues { pageInfo { totalCount } }
				}
			}
			staging: environment(name: "staging") {
				postgres(name: "foobar") { issues { pageInfo { totalCount } } }
			}
		}
		other: team(slug: "postgres-other-team") {
			issues(filter: { issueType: POSTGRES }) { pageInfo { totalCount } }
		}
	}]]
	t.check { data = {
		team = {
			environment = {
				postgres = {
					issues = {
						pageInfo = { totalCount = 1 }, nodes = { { __typename = "PostgresIssue" } },
						facets = {
							resourceTypes = { { resourceType = "POSTGRES", count = 1 } },
							issueTypes = { { issueType = "POSTGRES", count = 1 } },
						},
					},
				},
				other = { issues = { pageInfo = { totalCount = 0 } } },
			},
			staging = { postgres = { issues = { pageInfo = { totalCount = 0 } } } },
		},
		other = { issues = { pageInfo = { totalCount = 0 } } },
	} }
end)

-- Emulate an API rollout before the new checker has refreshed the old snapshot.
Helper.SQLExec([[
	UPDATE issues SET issue_type = 'POSTGRES_BRANCH', resource_type = 'POSTGRES_BRANCH',
		resource_name = 'broken-main-69d0ff6e'
	WHERE issue_type = 'POSTGRES' AND resource_name = 'broken' AND team = 'someteamname' AND env = 'dev'
]])

Test.gql("Old stored rows are exposed as PostgresIssue with the same Node ID", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[{
		node(id: "%s") {
			__typename
			... on PostgresIssue { id postgres { name teamEnvironment { name team { slug } } } }
		}
		team(slug: "someteamname") {
			issues(filter: { resourceName: "broken-main-69d0ff6e" }) { pageInfo { totalCount } nodes { __typename ... on PostgresIssue { postgres { name } } } }
		}
	}]], State.postgres_issue_id))
	t.check { data = {
		node = {
			__typename = "PostgresIssue", id = State.postgres_issue_id,
			postgres = { name = "broken", teamEnvironment = { name = "dev", team = { slug = "someteamname" } } },
		},
		team = { issues = { pageInfo = { totalCount = 1 }, nodes = { { __typename = "PostgresIssue", postgres = { name = "broken" } } } } },
	} }
end)

local function teamPostgresFacets(count)
	-- Team facets intentionally retain unrelated options with zero matches.
	return {
		resourceTypes = { { resourceType = "APPLICATION", count = 0 }, { resourceType = "JOB", count = 0 }, { resourceType = "POSTGRES", count = count } },
		issueTypes = { { issueType = "DEPRECATED_REGISTRY", count = 0 }, { issueType = "NO_RUNNING_INSTANCES", count = 0 }, { issueType = "POSTGRES", count = count } },
	}
end

for _, filter in ipairs({ "resourceType: POSTGRES", "issueType: POSTGRES", "resourceType: POSTGRES, issueType: POSTGRES" }) do
	Test.gql("Legacy rows match canonical issue filters before refresh: " .. filter, function(t)
		t.addHeader("x-user-email", user:email())
		t.query([[{ team(slug: "someteamname") {
			issues(filter: { ]] .. filter .. [[ }) {
				pageInfo { totalCount }
				nodes { __typename id ... on PostgresIssue { postgres { name } } }
				facets { resourceTypes { resourceType count } issueTypes { issueType count } }
			}
		} }]])
		t.check { data = { team = { issues = {
			pageInfo = { totalCount = 1 },
			nodes = { { __typename = "PostgresIssue", id = State.postgres_issue_id, postgres = { name = "broken" } } },
			facets = teamPostgresFacets(1),
		} } } }
	end)
end

Test.gql("Legacy rows retain logical resource filters and database scopes before refresh", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug: "someteamname") {
		issues(filter: { resourceType: POSTGRES, issueType: POSTGRES, resourceName: "broken", environments: ["dev"] }) {
			pageInfo { totalCount } nodes { __typename }
			facets { resourceTypes { resourceType count } issueTypes { issueType count } }
		}
		environment(name: "dev") {
			postgres(name: "broken") {
				issues(filter: { issueType: POSTGRES }) {
					pageInfo { totalCount } nodes { __typename }
					facets { resourceTypes { resourceType count } issueTypes { issueType count } }
				}
			}
			other: postgres(name: "another-db") { issues { pageInfo { totalCount } } }
			filteredOut: postgres(name: "broken") {
				issues(filter: { severity: WARNING }) { pageInfo { totalCount } facets { resourceTypes { resourceType count } issueTypes { issueType count } } }
			}
		}
		staging: environment(name: "staging") { postgres(name: "foobar") { issues { pageInfo { totalCount } } } }
	} }]]
	local expected = {
		pageInfo = { totalCount = 1 },
		nodes = { { __typename = "PostgresIssue" } },
		facets = { resourceTypes = { { resourceType = "POSTGRES", count = 1 } }, issueTypes = { { issueType = "POSTGRES", count = 1 } } },
	}
	t.check { data = { team = {
		issues = { pageInfo = expected.pageInfo, nodes = expected.nodes, facets = teamPostgresFacets(1) },
		environment = {
			postgres = { issues = expected },
			other = { issues = { pageInfo = { totalCount = 0 } } },
			filteredOut = { issues = {
				pageInfo = { totalCount = 0 },
				facets = { resourceTypes = { { resourceType = "POSTGRES", count = 0 } }, issueTypes = { { issueType = "POSTGRES", count = 0 } } },
			} },
		},
		staging = { postgres = { issues = { pageInfo = { totalCount = 0 } } } },
	} } }
end)

-- Mixed snapshots are also possible while old and new checker versions overlap.
Helper.SQLExec([[
	INSERT INTO issues (issue_type, resource_type, resource_name, team, env, severity, message)
	VALUES ('POSTGRES', 'POSTGRES', 'another-db', 'someteamname', 'dev', 'CRITICAL', 'Database failure')
]])

Test.gql("Mixed legacy and canonical rows aggregate into a single Postgres facet", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug: "someteamname") {
		issues(filter: { resourceType: POSTGRES, issueType: POSTGRES }) {
			pageInfo { totalCount }
			facets { resourceTypes { resourceType count } issueTypes { issueType count } severities { severity count } }
		}
	} }]]
	local facets = teamPostgresFacets(2)
	facets.severities = { { severity = "CRITICAL", count = 2 }, { severity = "WARNING", count = 0 } }
	t.check { data = { team = { issues = {
		pageInfo = { totalCount = 2 },
		facets = facets,
	} } } }
end)

checker:runChecks()

Test.sql("Checker replaces the legacy snapshot rather than duplicating issues", function(t)
	t.query [[SELECT issue_type, resource_type, resource_name FROM issues WHERE team = 'someteamname' AND issue_type IN ('POSTGRES', 'POSTGRES_BRANCH')]]
	t.check { { issue_type = "POSTGRES", resource_type = "POSTGRES", resource_name = "broken" } }
end)
