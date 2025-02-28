local team = Team.new("some-team", "purpose", "#channel")
local user1 = User.new()
local user2 = User.new()

team:addOwner(user1, user2)

Test.gql("Create delete key", function(t)
	t.addHeader("x-user-email", user1:email())

	t.query(string.format([[
		mutation {
			requestTeamDeletion(input: {
				slug: "%s"
			}) {
				key {
					key
				}
			}
		}
	]], team:slug()))

	t.check {
		data = {
			requestTeamDeletion = {
				key = {
					key = Save("key"),
				},
			},
		},
	}
end)

Test.gql("Confirm team deletion with the same user", function(t)
	t.addHeader("x-user-email", user1:email())

	t.query(string.format([[
		mutation {
			confirmTeamDeletion(input: {
				slug: "%s"
				key: "%s"
			}) {
				deletionStarted
			}
		}
	]], team:slug(), State.key))

	t.check {
		data = Null,
		errors = {
			{
				message = "You cannot confirm your own delete key.",
				path = { "confirmTeamDeletion" },
			},
		},
	}
end)

Test.gql("Confirm team deletion", function(t)
	t.addHeader("x-user-email", user2:email())

	t.query(string.format([[
		mutation {
			confirmTeamDeletion(input: {
				slug: "%s"
				key: "%s"
			}) {
				deletionStarted
			}
		}
	]], team:slug(), State.key))

	t.check {
		data = {
			confirmTeamDeletion = {
				deletionStarted = true,
			},
		},
	}
end)

Test.pubsub("Team deleted event", function(t)
	t.check("topic", {
		attributes = {
			CorrelationID = NotNull(),
			EventType = "EVENT_TEAM_DELETED",
		},
		data = {
			slug = team:slug(),
		},
	})
end)

Test.sql("Delete key confirmed", function(t)
	t.queryRow([[
		SELECT
  			team_slug
		FROM
  			team_delete_keys
		WHERE
  			key = $1
  			AND confirmed_at IS NOT NULL;
	]], State.key)

	t.check {
		team_slug = team:slug(),
	}
end)
