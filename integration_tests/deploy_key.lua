local user = User.new("name", "email@example", "extid")
local nonMember = User.new("other", "email@f.com", "extid2")


Test.gql("Create team", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			createTeam(
				input: {
					slug: "newteam"
					purpose: "some purpose"
					slackChannel: "#channel"
				}
			) {
				team {
					slug
				}
			}
		}
	]]

	t.check {
		data = {
			createTeam = {
				team = {
					slug = "newteam",
				},
			},
		},
	}
end)

Test.gql("Get deploy key for team I'm member of", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
	{
		team(slug: "newteam") {
			deploymentKey {
				id
				key
				created
				expires
			}
		}
	}
	]]

	t.check {
		data = {
			team = {
				deploymentKey = {
					id      = "DK_5BePKzWsvC",
					key     = Save("key"),
					created = NotNull(),
					expires = NotNull(),
				},
			},
		},
	}
end)

Test.gql("Get deploy key for team not member of", function(t)
	t.addHeader("x-user-email", nonMember:email())

	t.query([[
		{
			team(slug: "newteam") {
				deploymentKey {
					id
					key
					created
					expires
				}
			}
		}
	]])

	t.check {
		data = {
			team = {
				deploymentKey = Null,
			},
		},
		errors = {
			{
				message = Contains("you need the \"deploy_key:read\""),
				path = { "team", "deploymentKey" },
			},
		},
	}
end)

Test.gql("Change deploy key as member", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			changeDeploymentKey(
				input: {
					teamSlug: "newteam"
				}
			) {
				deploymentKey {
					id
					key
					created
					expires
				}
			}
		}
	]]

	t.check {
		data = {
			changeDeploymentKey = {
				deploymentKey = {
					id      = "DK_5BePKzWsvC",
					key     = Save("key"),
					created = NotNull(),
					expires = NotNull(),
				},
			},
		},
	}
end)

Test.gql("Check activity log after deploy key change", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "newteam") {
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
						{ message = "Updated deployment key" },
						{ message = "Created team" },
					},
				},
			},
		},
	}
end)

Test.gql("Change deploy key as non-member", function(t)
	t.addHeader("x-user-email", nonMember:email())

	t.query([[
		mutation {
			changeDeploymentKey(
				input: {
					teamSlug: "newteam"
				}
			) {
				deploymentKey {
					id
					key
					created
					expires
				}
			}
		}
	]])

	t.check {
		data = Null,
		errors = {
			{
				message = Contains("you need the \"deploy_key:update\""),
				path = { "changeDeploymentKey" },
			},
		},
	}
end)
