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
	local row = Helper.SQLQueryRow([[
		INSERT INTO deployments (team_slug, repository, environment_name)
		VALUES ('slug-2', CONCAT('org/repo-', $1::text), 'dev')
		RETURNING id::text
	]], tostring(i))

	Helper.SQLExec([[
		INSERT INTO deployment_k8s_resources(deployment_id, "group", version, kind, name, namespace)
		VALUES ($1, 'grp-1', 'v1', 'kind-1', 'name-1', 'slug-2'),
		($1, 'grp-2', 'v2', 'kind-2', 'name-2', 'slug-2');
	]], row.id)

	Helper.SQLExec([[
		INSERT INTO deployment_statuses (deployment_id, state, message)
		VALUES ($1, 'success', 'Deployment successful');
	]], row.id)
end

Test.gql("team with deployments", function(t)
	t.query([[
		{
			team(slug: "slug-2") {
				deployments(first: 1, after: "42E5H9") {
					nodes {
						id
						createdAt
						teamSlug
						resources {
							nodes {
								id
								kind
								name
							}
						}
						statuses {
							nodes {
								id
								createdAt
								state
								message
							}
						}
						repository
						environmentName
					}
					edges {
						node {
							repository
							environmentName
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
						{
							id = NotNull(),
							createdAt = NotNull(),
							resources = {
								nodes = {
									{ id = NotNull(), kind = "kind-2", name = "name-2" },
									{ id = NotNull(), kind = "kind-1", name = "name-1" },
								},
							},
							statuses = {
								nodes = {
									{ id = NotNull(), createdAt = NotNull(), state = "SUCCESS", message = "Deployment successful" },
								},
							},
							repository = "org/repo-2",
							environmentName = "dev",
							teamSlug = "slug-2",
						},
					},
					edges = {
						{ node = { repository = "org/repo-2", environmentName = "dev" } },
					},
					pageInfo = {
						hasNextPage = true,
						hasPreviousPage = true,
						startCursor = "42E5HA",
						endCursor = "42E5HA",
						totalCount = 3,
					},
				},
			},
		},
	}
end)
