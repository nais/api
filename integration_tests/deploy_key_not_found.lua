local user = User.new("name", "email@example", "extid")

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

Test.gql("Get deploy key for team with no deploy key returns null without error", function(t)
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
				deploymentKey = Null,
			},
		},
	}
end)
