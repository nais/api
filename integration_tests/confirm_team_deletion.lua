local team = Team.new("some-team", "purpose", "#channel")
local user1 = User.new()
local user2 = User.new()

team:addOwner(user1, user2)

local serviceAccount = ServiceAccount.new("team-owner", team:slug())
serviceAccount:assignRole("Team owner")

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
				locations = NotNull(),
				message = "You cannot confirm your own delete key.",
				path = { "confirmTeamDeletion" },
			},
		},
	}
end)

Test.gql("Service account owner cannot confirm team deletion", function(t)
	t.addHeader("authorization", "Bearer " .. serviceAccount:token())

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
				locations = NotNull(),
				message = "You are authenticated, but your account is not authorized to perform this action.",
				path = { "confirmTeamDeletion" },
			},
		},
	}
end)

Test.sql("Rejected service account confirmation leaves deletion pending", function(t)
	t.queryRow([[
		SELECT
			team_delete_keys.confirmed_at,
			teams.delete_key_confirmed_at,
			(SELECT COUNT(*) FROM activity_log_entries WHERE team_slug = $2 AND action = 'CONFIRM_DELETE_KEY') AS activity_count
		FROM team_delete_keys
		JOIN teams ON teams.slug = team_delete_keys.team_slug
		WHERE team_delete_keys.key = $1
	]], State.key, team:slug())

	t.check {
		confirmed_at = Null,
		delete_key_confirmed_at = Null,
		activity_count = 0,
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
