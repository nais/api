Helper.readK8sResources("k8s_resources/simple")

local user = User.new()
local nonMember = User.new()
local team = Team.new("slug-1", "purpose", "#channel")
team:addMember(user)

Test.gql("application list", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "slug-1") {
				applications {
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
				applications = {
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
			deleteApplication(
				input: {teamSlug: "slug-1", environmentName: "dev", name: "%s"}
			) {
				success
			}
		}
	]], State.app1))

	t.check {
		data = {
			deleteApplication = {
				success = true,
			},
		},
	}
end)

Test.gql("as non-team member", function(t)
	t.addHeader("x-user-email", nonMember:email())

	t.query(string.format([[
		mutation {
			deleteApplication(
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
				message = Contains("you need the \"applications:delete\""),
				path = { "deleteApplication" },
			},
		},
	}
end)

Test.gql("application list after deletion", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "slug-1") {
				applications {
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
				applications = {
					nodes = {
						{ name = "app-name" },
					},
					pageInfo = {
						totalCount = 1,
					},
				},
			},
		},
	}
end)
