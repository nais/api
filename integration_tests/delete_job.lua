Helper.readK8sResources("k8s_resources/simple")

local user = User.new()
local nonMember = User.new()
local team = Team.new("slug-1", "purpose", "#channel")
team:addMember(user)

Test.gql("job list", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "slug-1") {
				jobs {
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
				jobs = {
					nodes = {
						{ name = Save("app1") },
						{ name = Save("app2") },
					},
					pageInfo = {
						totalCount = 2,
					},
				},
			},
		},
	}
end)

Test.gql("as team member", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		mutation {
			deleteJob(
				input: {teamSlug: "slug-1", environmentName: "dev", name: "%s"}
			) {
				success
			}
		}
	]], State.app1))

	t.check {
		data = {
			deleteJob = {
				success = true,
			},
		},
	}
end)

Test.gql("as non-team member", function(t)
	t.addHeader("x-user-email", nonMember:email())

	t.query(string.format([[
		mutation {
			deleteJob(
				input: {teamSlug: "slug-1", environmentName: "dev", name: "%s"}
			) {
				success
			}
		}
	]], State.app2))

	t.check {
		data = Null,
		errors = {
			{
				message = Contains("you need the \"jobs:delete\""),
				path = { "deleteJob" },
			},
		},
	}
end)

Test.gql("job list after deletion", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "slug-1") {
				jobs {
					nodes {
						name
					}
					pageInfo {
						totalCount
					}
				}
				activityLog {
					nodes {
						message
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
							message = "Job deleted",
						},
					},
				},
				jobs = {
					nodes = {
						{ name = "jobname-2" },
					},
					pageInfo = {
						totalCount = 1,
					},
				},
			},
		},
	}
end)
