Test.gql("team with no deployments", function(t)
	t.query([[
		query {
			team(slug: "slug-1") {
				deployments {
					nodes {
						id
					}
					edges {
						node {
							id
						}
					}
					pageInfo {
						hasNextPage
						hasPreviousPage
						startCursor
						endCursor
						totalCount
					}
				}
			}
		}
	]])

	t.check {
		data = {
			team = {
				deployments = {
					nodes = {},
					edges = {},
					pageInfo = {
						hasNextPage = false,
						hasPreviousPage = false,
						startCursor = Null,
						endCursor = Null,
						totalCount = 0,
					},
				},
			},
		},
	}
end)

-- Create a few deployments
for i = 1, 3, 1 do
	Helper.SQLExec([[
		INSERT INTO deployments (team_slug,	repository,	environment_name)
		VALUES ('slug-2', CONCAT('org/repo-', $1::text), 'dev');
	]], tostring(i))
end

Test.gql("team with deployments", function(t)
	t.query([[
		query {
			team(slug: "slug-2") {
				deployments {
					nodes {
						repository
						environment { name }
					}
					edges {
						node {
							repository
							environment { name }
						}
					}
					pageInfo {
						hasNextPage
						hasPreviousPage
						startCursor
						endCursor
						totalCount
					}
				}
			}
		}
	]])

	t.check {
		data = {
			team = {
				deployments = {
					nodes = {
						{ repository = "org/repo-3", environment = { name = "dev" } },
						{ repository = "org/repo-2", environment = { name = "dev" } },
						{ repository = "org/repo-1", environment = { name = "dev" } },
					},
					edges = {
						{ node = { repository = "org/repo-3", environment = { name = "dev" } } },
						{ node = { repository = "org/repo-2", environment = { name = "dev" } } },
						{ node = { repository = "org/repo-1", environment = { name = "dev" } } },
					},
					pageInfo = {
						hasNextPage = false,
						hasPreviousPage = false,
						startCursor = "42E5H9",
						endCursor = "42E5HB",
						totalCount = 3,
					},
				},
			},
		},
	}
end)
