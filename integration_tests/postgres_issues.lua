local user = User.new("user", "user@usersen.com")
local team = Team.new("someteamname", "purpose", "#slack_channel")
team:addMember(user)
Team.new("postgres-other-team", "Isolation", "#postgres")
Helper.readK8sResources("k8s_resources/postgres_branches")
local checker = IssueChecker.new()
checker:runChecks()

Test.gql("Degraded Postgres produces an issue with its legacy identity", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{
		team(slug: "someteamname") {
			issues(filter: { issueType: POSTGRES_BRANCH, resourceType: POSTGRES_BRANCH }) {
				pageInfo { totalCount }
				nodes {
					__typename id severity message
					teamEnvironment { name }
					... on PostgresBranchIssue {
						branch { name state postgres { name activeBranch { name } } }
					}
				}
			}
		}
	}]]
	t.check { data = { team = { issues = {
		pageInfo = { totalCount = 1 },
		nodes = { {
			__typename = "PostgresBranchIssue",
			id = NotNull(),
			severity = "CRITICAL",
			message = 'Postgres "broken" has reported a failure.',
			teamEnvironment = { name = "dev" },
			branch = { name = "main", state = "DEGRADED", postgres = { name = "broken", activeBranch = { name = "main" } } },
		} },
	} } } }
end)

Test.gql("Branch issues are scoped to the concrete branch", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{
		team(slug: "someteamname") {
			environment(name: "dev") {
				postgres(name: "broken") {
					issues(filter: { severity: CRITICAL }) {
						pageInfo { totalCount } nodes { __typename }
						facets { resourceTypes { resourceType count } issueTypes { issueType count } }
					}
					main: branch(name: "main") { issues { pageInfo { totalCount } } }
				}
				other: postgres(name: "another-db") {
					branch(name: "main") { issues { pageInfo { totalCount } } }
				}
			}
			staging: environment(name: "staging") {
				postgres(name: "foobar") { branch(name: "main") { issues { pageInfo { totalCount } } } }
			}
		}
		other: team(slug: "postgres-other-team") {
			issues(filter: { issueType: POSTGRES_BRANCH }) { pageInfo { totalCount } }
		}
	}]]
	t.check { data = {
		team = {
			environment = {
				postgres = {
					issues = {
						pageInfo = { totalCount = 1 }, nodes = { { __typename = "PostgresBranchIssue" } },
						facets = {
							resourceTypes = { { resourceType = "POSTGRES_BRANCH", count = 1 } },
							issueTypes = { { issueType = "POSTGRES_BRANCH", count = 1 } },
						},
					},
					main = { issues = { pageInfo = { totalCount = 1 } } },
				},
				other = { branch = { issues = { pageInfo = { totalCount = 0 } } } },
			},
			staging = { postgres = { branch = { issues = { pageInfo = { totalCount = 0 } } } } },
		},
		other = { issues = { pageInfo = { totalCount = 0 } } },
	} }
end)
