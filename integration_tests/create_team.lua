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
					id
					slug
				}
			}
		}
	]]

	t.check {
		data = {
			createTeam = {
				team = {
					id = Save("teamID"),
					slug = "newteam"
				}
			}
		}
	}
end)


Test.pubsub("Check if pubsub message was sent", function(t)
	t.check("topic", {
		attributes = {
			CorrelationID = NotNull(),
			EventType = "EVENT_TEAM_CREATED"
		},
		data = {
			slug = "newteam"
		}
	})
end)

Test.sql("Check database", function(t)
	t.queryRow ("SELECT * FROM teams WHERE slug = $1", "newteam")

	t.check {
		azure_group_id = Null,
		gar_repository = Null,
		github_team_slug = Null,
		google_group_email = Null,
		last_successful_sync = Null,
		cdn_bucket = Null,
		delete_key_confirmed_at = Null,
		purpose = "some purpose",
		slack_channel = "#channel",
		slug = "newteam"
	}
end)

Test.gql("Team node query", function(t)
	t.query(string.format([[
		query {
			node(id: "%s") {
				... on Team {
					slug
				}
			}
		}
	]], State.teamID))

	t.check {
		data = {
			node = {
				slug = "newteam"
			}
		}
	}
end)
