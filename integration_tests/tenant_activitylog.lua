local admin = User.new()
admin:admin(true)
local teamOne = Team.new("slug-1", "purpose", "#channel")
local teamTwo = Team.new("slug-2", "purpose", "#channel")

Test.gql("Create repository event for team one", function(t)
	t.addHeader("x-user-email", admin:email())

	t.query [[
		mutation {
			addRepositoryToTeam(input: {teamSlug: "slug-1", repositoryName: "nais/api-team-one"}) {
				repository {
					name
				}
			}
		}
	]]

	t.check {
		data = {
			addRepositoryToTeam = {
				repository = {
					name = "nais/api-team-one",
				},
			},
		},
	}
end)

Test.gql("Create repository event for team two", function(t)
	t.addHeader("x-user-email", admin:email())

	t.query [[
		mutation {
			addRepositoryToTeam(input: {teamSlug: "slug-2", repositoryName: "nais/api-team-two"}) {
				repository {
					name
				}
			}
		}
	]]

	t.check {
		data = {
			addRepositoryToTeam = {
				repository = {
					name = "nais/api-team-two",
				},
			},
		},
	}
end)

Helper.SQLExec([[
	INSERT INTO activity_log_entries (actor, action, resource_type, resource_name, team_slug, environment, data)
	VALUES ('deployer', 'DEPLOYMENT', 'APP', 'deployed-app', 'slug-1', 'dev', '{}')
]])

Test.gql("Tenant activity log includes deployments", function(t)
	t.addHeader("x-user-email", admin:email())

	t.query [[
		query {
			activityLog(first: 10, filter: { activityTypes: [DEPLOYMENT] }) {
				nodes {
					resourceName
				}
				pageInfo {
					totalCount
				}
				facets {
					activityTypes {
						activityType
						count
					}
					resourceTypes {
						resourceType
						count
					}
					environments {
						value
						count
					}
				}
			}
		}
	]]

	t.check {
		data = {
			activityLog = {
				nodes = {
					{
						resourceName = "deployed-app",
					},
				},
				pageInfo = {
					totalCount = 1,
				},
				facets = {
					activityTypes = {
						{
							activityType = "DEPLOYMENT",
							count = 1,
						},
						{
							activityType = "REPOSITORY_ADDED",
							count = 0,
						},
					},
					resourceTypes = {
						{
							resourceType = "APP",
							count = 1,
						},
						{
							resourceType = "REPOSITORY",
							count = 0,
						},
					},
					environments = {
						{
							value = "dev",
							count = 1,
						},
					},
				},
			},
		},
	}
end)

Test.gql("Tenant activity log returns facets and pagination metadata", function(t)
	t.addHeader("x-user-email", admin:email())

	t.query [[
		query {
			activityLog(first: 10, filter: { activityTypes: [REPOSITORY_ADDED] }) {
				nodes {
					resourceName
					teamSlug
				}
				pageInfo {
					totalCount
					hasNextPage
				}
				facets {
					activityTypes {
						activityType
						count
					}
					resourceTypes {
						resourceType
						count
					}
					environments {
						value
						count
					}
				}
			}
		}
	]]

	t.check {
		data = {
			activityLog = {
				nodes = {
					{
						resourceName = "nais/api-team-two",
						teamSlug = "slug-2",
					},
					{
						resourceName = "nais/api-team-one",
						teamSlug = "slug-1",
					},
				},
				pageInfo = {
					totalCount = 2,
					hasNextPage = false,
				},
				facets = {
					activityTypes = {
						{
							activityType = "DEPLOYMENT",
							count = 0,
						},
						{
							activityType = "REPOSITORY_ADDED",
							count = 2,
						},
					},
					resourceTypes = {
						{
							resourceType = "APP",
							count = 0,
						},
						{
							resourceType = "REPOSITORY",
							count = 2,
						},
					},
					environments = {
						{
							value = "dev",
							count = 0,
						},
					},
				},
			},
		},
	}
end)

Test.gql("Tenant activity log supports time filtering", function(t)
	t.addHeader("x-user-email", admin:email())

	t.query [[
		query {
			activityLog(
				first: 10
				filter: { activityTypes: [REPOSITORY_ADDED], from: "9999-01-01T00:00:00Z" }
			) {
				nodes {
					resourceName
				}
				pageInfo {
					totalCount
				}
			}
		}
	]]

	t.check {
		data = {
			activityLog = {
				nodes = {},
				pageInfo = {
					totalCount = 0,
				},
			},
		},
	}
end)
