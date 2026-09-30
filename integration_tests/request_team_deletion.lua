local team = Team.new("delete-me", "Delete me", "#delete-me")
local user = User.new("Authenticated User", "auth@example.com", "auth-external-id")
team:addOwner(user)

local otherUser = User.new("Other User", "other@example.com", "other")
local serviceAccount = ServiceAccount.new("team-owner", team:slug())
serviceAccount:assignRole("Team owner")

local deleteKey = Helper.SQLQueryRow([[
	INSERT INTO team_delete_keys (
		team_slug,
		created_by
	) VALUES (
		$1,
		(SELECT id FROM users WHERE email = $2)
	) RETURNING key::TEXT;
]], team:slug(), otherUser:email())

Test.gql("Service account owner cannot request team deletion", function(t)
	t.addHeader("authorization", "Bearer " .. serviceAccount:token())

	t.query(string.format([[
		mutation {
			requestTeamDeletion(input: { slug: "%s" }) {
				key { key }
			}
		}
	]], team:slug()))

	t.check {
		data = Null,
		errors = {
			{
				locations = NotNull(),
				message = "You are authenticated, but your account is not authorized to perform this action.",
				path = { "requestTeamDeletion" },
			},
		},
	}
end)

Test.sql("Rejected service account request creates no key or activity log", function(t)
	t.queryRow([[
		SELECT
			(SELECT COUNT(*) FROM team_delete_keys WHERE team_slug = $1) AS key_count,
			(SELECT COUNT(*) FROM activity_log_entries WHERE team_slug = $1 AND action = 'CREATE_DELETE_KEY') AS activity_count
	]], team:slug())

	t.check {
		key_count = 1,
		activity_count = 0,
	}
end)

Test.gql("Request team deletion", function(t)
	t.addHeader("x-user-email", user:email())

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
					key = Save("deleteKey"),
				},
			},
		},
	}
end)

Test.sql("Validate delete key", function(t)
	t.queryRow([[
		SELECT
			team_delete_keys.confirmed_at,
			team_delete_keys.team_slug,
			users.email
		FROM
			team_delete_keys
		JOIN
			users ON users.id = team_delete_keys.created_by
		WHERE key = $1;
	]], State.deleteKey)

	t.check {
		confirmed_at = Null,
		team_slug = team:slug(),
		email = user:email(),
	}
end)
