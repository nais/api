TeamSlug = "someteamname"

Test.gql("Create team", function(t)
	t.query(string.format([[
		mutation {
			createTeam(
				input: {
					slug: "%s"
					purpose: "some purpose"
					slackChannel: "#default"
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

Test.gql("Update environment", function(t)
	t.query(string.format([[
		mutation {
			updateTeamEnvironment(
				input: {
					slug: "%s"
					environmentName: "dev"
					slackAlertsChannel:"#dev"
				}
			) {
				environment {
					name
					team {
						slug
						environments {
							name
							slackAlertsChannel
						}
					}
				}
			}
		}
	]], TeamSlug))

	t.check {
		data = {
			updateTeamEnvironment = {
				environment = {
					name = "dev",
					team = {
						slug = TeamSlug,
						environments = {
							{
								name = "dev",
								slackAlertsChannel = "#dev"
							},
							{
								name = "dev-fss",
								slackAlertsChannel = "#default"
							},
							{
								name = "dev-gcp",
								slackAlertsChannel = "#default"
							},
							{
								name = "staging",
								slackAlertsChannel = "#default"
							}
						}
					}
				}
			}
		}
	}
end)

Test.gql("Invalid channel name", function(t)
	t.query(string.format([[
		mutation {
			updateTeamEnvironment(
				input: {
					slug: "%s"
					environmentName: "dev"
					slackAlertsChannel:"dev"
				}
			) {
				environment {
					name
				}
			}
		}
	]], TeamSlug))

	t.check {
		data = Null,
		errors = {
			{
				extensions = {
					field = "slackAlertsChannel",
					rule = "optionalslackchannel"
				},
				message = "\"dev\" is not a valid Slack channel name. A valid channel name starts with a '#' and is between 3 and 80 characters long.",
				path = {
					"updateTeamEnvironment"
				}
			}
		}
	}
end)

Test.gql("Nothing to update", function(t)
	t.query(string.format([[
		mutation {
			updateTeamEnvironment(
				input: {
					slug: "%s"
					environmentName: "dev"
				}
			) {
				environment {
					name
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
					"updateTeamEnvironment"
				}
			}
		}
	}
end)

Test.gql("Remove channel", function(t)
	t.query(string.format([[
		mutation {
			updateTeamEnvironment(
				input: {
					slug: "%s"
					environmentName: "dev"
					slackAlertsChannel: ""
				}
			) {
				environment {
					name
					team {
						slug
						environments {
							name
							slackAlertsChannel
						}
					}
				}
			}
		}
	]], TeamSlug))

	t.check {
		data = {
			updateTeamEnvironment = {
				environment = {
					name = "dev",
					team = {
						slug = TeamSlug,
						environments = {
							{
								name = "dev",
								slackAlertsChannel = "#default"
							},
							{
								name = "dev-fss",
								slackAlertsChannel = "#default"
							},
							{
								name = "dev-gcp",
								slackAlertsChannel = "#default"
							},
							{
								name = "staging",
								slackAlertsChannel = "#default"
							}
						}
					}
				}
			}
		}
	}
end)
