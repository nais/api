local team = Team.new("delete-me", "Delete me", "#delete-me")
local user = User.new("Authenticated User", "auth@example.com", "auth-external-id")
team:addOwner(user)

local otherUser = User.new("Other User", "other@example.com", "other")

local deleteKey = Helper.SQLQueryRow([[
	INSERT INTO team_delete_keys (
		team_slug,
		created_by
	) VALUES (
		$1,
		(SELECT id FROM users WHERE email = $2)
	) RETURNING key::TEXT;
]], team:slug(), otherUser:email())

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
