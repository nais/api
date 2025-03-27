local admin = User.new()
admin:admin(true)
local user = User.new()
local nonMember = User.new()
local team = Team.new("slug-1", "purpose", "#channel")
team:addMember(user)

Test.gql("List repositories for team", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		{
			team(slug: "slug-1") {
				repositories {
					nodes {
						name
					}
					pageInfo {
						totalCount
					}
				}
			}
		}
	]]

	t.check {
		data = {
			team = {
				repositories = {
					nodes = {},
					pageInfo = {
						totalCount = 0,
					},
				},
			},
		},
	}
end)

Test.gql("Add repository to team as team member", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			addRepositoryToTeam(input: {teamSlug: "slug-1", repositoryName: "nais/api"}) {
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
					name = "nais/api",
				},
			},
		},
	}
end)

Test.gql("Add repository to non-existing-team as admin", function(t)
	t.addHeader("x-user-email", admin:email())

	t.query [[
		mutation {
			addRepositoryToTeam(input: {teamSlug: "team-that-does-not-exist", repositoryName: "nais/api"}) {
				repository {
					name
				}
			}
		}
	]]

	t.check {
		data = Null,
		errors = {
			{
				message = "The specified team was not found.",
				path = { "addRepositoryToTeam" },
			},
		},
	}
end)

Test.gql("Add repository to team as non-team member", function(t)
	t.addHeader("x-user-email", nonMember:email())

	t.query([[
		mutation {
			addRepositoryToTeam(input: {teamSlug: "slug-1", repositoryName: "nais/api"}) {
				repository {
					name
				}
			}
		}
	]])

	t.check {
		data = Null,
		errors = {
			{
				message = Contains("you need the \"repositories:create\""),
				path = { "addRepositoryToTeam" },
			},
		},
	}
end)

Test.gql("List repositories for team after creation", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		{
			team(slug: "slug-1") {
				activityLog {
					nodes {
						message
						resourceName
					}
				}
				repositories {
					nodes {
						name
					}
					pageInfo {
						totalCount
					}
				}
			}
		}
	]]

	t.check {
		data = {
			team = {
				activityLog = {
					nodes = {
						{
							message = "Added repository to team",
							resourceName = "nais/api",
						},
					},
				},
				repositories = {
					nodes = {
						{
							name = "nais/api",
						},
					},
					pageInfo = {
						totalCount = 1,
					},
				},
			},
		},
	}
end)

Test.gql("Remove repository from team as non-team member", function(t)
	t.addHeader("x-user-email", nonMember:email())

	t.query([[
		mutation {
			removeRepositoryFromTeam(input: {teamSlug: "slug-1", repositoryName: "nais/api"}) {
				success
			}
		}
	]])

	t.check {
		data = Null,
		errors = {
			{
				message = Contains("you need the \"repositories:delete\""),
				path = { "removeRepositoryFromTeam" },
			},
		},
	}
end)

Test.gql("Remove repository from team as team member", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			removeRepositoryFromTeam(input: {teamSlug: "slug-1", repositoryName: "nais/api"}) {
				success
			}
		}
	]]

	t.check {
		data = {
			removeRepositoryFromTeam = {
				success = true,
			},
		},
	}
end)

Test.gql("List repositories for team after deletion", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		{
			team(slug: "slug-1") {
				activityLog {
					nodes {
						message
						resourceName
					}
				}
				repositories {
					nodes {
						name
					}
					pageInfo {
						totalCount
					}
				}
			}
		}
	]]

	t.check {
		data = {
			team = {
				activityLog = {
					nodes = {
						{
							message = "Removed repository from team",
							resourceName = "nais/api",
						},
						{
							message = "Added repository to team",
							resourceName = "nais/api",
						},
					},
				},
				repositories = {
					nodes = {},
					pageInfo = {
						totalCount = 0,
					},
				},
			},
		},
	}
end)
