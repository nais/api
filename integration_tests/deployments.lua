Helper.readK8sResources("k8s_resources/simple")

local user = User.new("usersen", "usr@exam.com", "ei")
Team.new("slug-1", "purpose", "#slack-channel")
Team.new("slug-2", "purpose", "#slack-channel")

Test.gql("team with no deployments", function(t)
	t.addHeader("x-user-email", user:email())

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
		VALUES ($1, 'nais.io', 'v1alpha1', 'Application', 'app-name', 'slug-2'),
		($1, 'grp-2', 'v2', 'kind-2', 'name-2', 'slug-2');
	]], row.id)

	Helper.SQLExec([[
		INSERT INTO deployment_statuses (deployment_id, state, message)
		VALUES ($1, 'success', 'Deployment successful');
	]], row.id)
end

Test.gql("team with deployments", function(t)
	t.addHeader("x-user-email", user:email())

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
									{ id = NotNull(), kind = "kind-2",      name = "name-2" },
									{ id = NotNull(), kind = "Application", name = "app-name" },
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


Test.gql("application with deployments", function(t)
	t.addHeader("x-user-email", user:email())

	t.query([[
		{
			team(slug: "slug-2") {
				environment(name: "dev") {
					application(name: "app-name") {
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
			}
		}
	]])

	t.check {
		data = {
			team = {
				environment = {
					application = {
						deployments = {
							nodes = {
								{
									id = NotNull(),
									createdAt = NotNull(),
									resources = {
										nodes = {
											{ id = NotNull(), kind = "kind-2",      name = "name-2" },
											{ id = NotNull(), kind = "Application", name = "app-name" },
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
			},
		},
	}
end)

Test.gql("application with no deployments", function(t)
	t.addHeader("x-user-email", user:email())

	t.query([[
		{
			team(slug: "slug-1") {
				environment(name: "dev") {
					application(name: "app-name") {
						deployments(first: 1) {
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
			}
		}
	]])

	t.check {
		data = {
			team = {
				environment = {
					application = {
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
			},
		},
	}
end)

Helper.SQLExec([[
	INSERT INTO deployments (team_slug, repository, environment_name)
	VALUES ('slug-1', 'org/repo', 'dev')
]])

Test.gql("team deployment without resources and statuses", function(t)
	t.addHeader("x-user-email", user:email())

	t.query([[
		{
			team(slug: "slug-1") {
				deployments(first: 1) {
					nodes {
						id
						resources {
							nodes {
								id
							}
						}
						statuses {
							nodes {
								id
							}
						}
						environmentName
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
							resources = { nodes = {} },
							statuses = { nodes = {} },
							environmentName = "dev",
						},
					},
				},
			},
		},
	}
end)

Test.gql("list all deployments", function(t)
	t.addHeader("x-user-email", user:email())

	t.query([[
		{
			deployments {
				nodes {
					id
					environmentName
				}
			}
		}
	]])

	t.check {
		data = {
			deployments = {
				nodes = {
					{
						id = NotNull(),
						environmentName = "dev",
					},
					{
						id = NotNull(),
						environmentName = "dev",
					},
					{
						id = NotNull(),
						environmentName = "dev",
					},
					{
						id = NotNull(),
						environmentName = "dev",
					},
				},
			},
		},
	}
end)

-- Create additional test data for filter tests
Team.new("slug-3", "purpose", "#slack-channel")

Helper.SQLExec([[
	INSERT INTO deployments (team_slug, repository, environment_name)
	VALUES
		('slug-1', 'org/repo-prod', 'prod'),
		('slug-2', 'org/repo-staging', 'staging'),
		('slug-3', 'org/repo-dev', 'dev'),
		('slug-3', 'org/repo-prod2', 'prod')
]])

Test.gql("filter deployments by environments", function(t)
	t.addHeader("x-user-email", user:email())

	t.query([[
		{
			deployments(filter: { environments: ["prod"] }) {
				pageInfo {
					totalCount
				}
				nodes {
					id
					environmentName
				}
			}
		}
	]])

	t.check {
		data = {
			deployments = {
				pageInfo = {
					totalCount = 2,
				},
				nodes = {
					{
						id = NotNull(),
						environmentName = "prod",
					},
					{
						id = NotNull(),
						environmentName = "prod",
					},
				},
			},
		},
	}
end)

Test.gql("filter deployments by multiple environments", function(t)
	t.addHeader("x-user-email", user:email())

	t.query([[
		{
			deployments(filter: { environments: ["prod", "staging"] }) {
				pageInfo {
					totalCount
				}
			}
		}
	]])

	t.check {
		data = {
			deployments = {
				pageInfo = {
					totalCount = 3,
				},
			},
		},
	}
end)

Test.gql("exclude teams from deployments", function(t)
	t.addHeader("x-user-email", user:email())

	t.query([[
		{
			deployments(filter: { excludeTeams: ["slug-1", "slug-2"] }) {
				pageInfo {
					totalCount
				}
			}
		}
	]])

	t.check {
		data = {
			deployments = {
				pageInfo = {
					totalCount = 2,
				},
			},
		},
	}
end)

Test.gql("combine environment and excludeTeams filters", function(t)
	t.addHeader("x-user-email", user:email())

	t.query([[
		{
			deployments(filter: { environments: ["dev"], excludeTeams: ["slug-2"] }) {
				pageInfo {
					totalCount
				}
			}
		}
	]])

	t.check {
		data = {
			deployments = {
				pageInfo = {
					totalCount = 2,
				},
			},
		},
	}
end)

Test.gql("empty environment filter returns all deployments", function(t)
	t.addHeader("x-user-email", user:email())

	t.query([[
		{
			deployments(filter: { environments: [] }) {
				pageInfo {
					totalCount
				}
			}
		}
	]])

	t.check {
		data = {
			deployments = {
				pageInfo = {
					totalCount = 8,
				},
			},
		},
	}
end)

Test.gql("empty excludeTeams filter returns all deployments", function(t)
	t.addHeader("x-user-email", user:email())

	t.query([[
		{
			deployments(filter: { excludeTeams: [] }) {
				pageInfo {
					totalCount
				}
			}
		}
	]])

	t.check {
		data = {
			deployments = {
				pageInfo = {
					totalCount = 8,
				},
			},
		},
	}
end)
