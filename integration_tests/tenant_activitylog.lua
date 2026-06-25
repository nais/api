local admin = User.new()
admin:admin(true)
local member = User.new()
local nonMember = User.new()
local teamOne = Team.new("slug-1", "purpose", "#channel")
local teamTwo = Team.new("slug-2", "purpose", "#channel")
teamOne:addMember(member)
teamTwo:addMember(member)

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

Test.gql("Tenant activity log returns facets with teams and pagination metadata", function(t)
	t.addHeader("x-user-email", admin:email())

	t.query [[
		query {
			tenantActivityLog(first: 10, filter: { activityTypes: [REPOSITORY_ADDED] }) {
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
					teams {
						value
						count
					}
				}
			}
		}
	]]

	t.check {
		data = {
			tenantActivityLog = {
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
							activityType = "REPOSITORY_ADDED",
							count = 2,
						},
					},
					resourceTypes = {
						{
							resourceType = "REPOSITORY",
							count = 2,
						},
					},
					environments = {},
					teams = {
						{
							value = "slug-1",
							count = 1,
						},
						{
							value = "slug-2",
							count = 1,
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
			tenantActivityLog(
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
			tenantActivityLog = {
				nodes = {},
				pageInfo = {
					totalCount = 0,
				},
			},
		},
	}
end)

Test.gql("Tenant activity log requires activity_logs read authorization", function(t)
	t.addHeader("x-user-email", nonMember:email())

	t.query [[
		query {
			tenantActivityLog(first: 1) {
				nodes {
					id
				}
			}
		}
	]]

	t.check {
		data = Null,
		errors = {
			{
				locations = NotNull(),
				message = Contains('you need the "activity_logs:read"'),
				path = { "tenantActivityLog" },
			},
		},
	}
end)
