Test.gql("Create team", function(t)
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
	]], { ["x-user-email"] = "email-12@example.com", })

	t.check {
		data = {
			team = {
				deploymentKey = Null,
			},
		},
		errors = {
			{
				message = Contains("You are authenticated, but your account is not authorized to perform this action"),
				path = { "team", "deploymentKey", },
			},
		},
	}
end)
