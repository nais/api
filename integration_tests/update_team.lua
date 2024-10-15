TeamSlug = "someteamname"

Test.gql("Create team", function(t)
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
	]], TeamSlug))

	t.check {
		data = {
			createTeam = {
				team = {
					slug = TeamSlug
				}
			}
		}
	}
end)

Test.gql("Update team", function(t)
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
	]], TeamSlug))

	t.check {
		data = {
			updateTeam = {
				team = {
					purpose = "new-purpose",
					slackChannel = "#new-slack-channel",
					environments = {
						{
							name = "dev",
							slackAlertsChannel = "#new-slack-channel"
						},
						{
							name = "dev-fss",
							slackAlertsChannel = "#new-slack-channel"
						},
						{
							name = "dev-gcp",
							slackAlertsChannel = "#new-slack-channel"
						},
						{
							name = "staging",
							slackAlertsChannel = "#new-slack-channel"
						}
					}
				}
			}
		}
	}
end)

Test.gql("Nothing to update", function(t)
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
	]], TeamSlug))

	t.check {
		data = Null,
		errors = {
			{
				message = "Nothing to update.",
				path = {
					"updateTeam"
				}
			}
		}
	}
end)
