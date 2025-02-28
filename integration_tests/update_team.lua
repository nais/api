local teamSlug = "someteamname"

local user = User.new("test", "test@test.com", "exttest")

Test.gql("Create team", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		mutation {
			createTeam(
				input: {
					slug: "%s"
					purpose: "some purpose"
					slackChannel: "#channel"
				}
			) {
				team {
					slug
				}
			}
		}
	]], teamSlug))

	t.check {
		data = {
			createTeam = {
				team = {
					slug = teamSlug,
				},
			},
		},
	}
end)

Test.gql("Update team", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		mutation {
			updateTeam(
				input: {
					slug: "%s"
					purpose: "new-purpose"
					slackChannel: "#new-slack-channel"
				}
			) {
				team {
					purpose
					slackChannel
					environments {
						name
						slackAlertsChannel
					}
				}
			}
		}
	]], teamSlug))

	t.check {
		data = {
			updateTeam = {
				team = {
					purpose = "new-purpose",
					slackChannel = "#new-slack-channel",
					environments = {
						{
							name = "dev",
							slackAlertsChannel = "#new-slack-channel",
						},
						{
							name = "dev-fss",
							slackAlertsChannel = "#new-slack-channel",
						},
						{
							name = "dev-gcp",
							slackAlertsChannel = "#new-slack-channel",
						},
						{
							name = "staging",
							slackAlertsChannel = "#new-slack-channel",
						},
					},
				},
			},
		},
	}
end)

Test.gql("Nothing to update", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		mutation {
			updateTeam(
				input: {
					slug: "%s"
				}
			) {
				team {
					purpose
				}
			}
		}
	]], teamSlug))

	t.check {
		data = {
			updateTeam = {
				team = {
					purpose = "new-purpose",
				},
			},
		},
	}
end)

-- TODO(chredvar): Add tests for invalid input for create and update team
