TeamSlug = "delete-me"

Test.sql("Initialize team", function(t)
	Helper.SQLExec([[
		INSERT INTO teams(
			slug,
			purpose,
			slack_channel
		) VALUES ($1, $2, $3);
	]], TeamSlug, "Delete me", "#delete-me")

	Helper.SQLExec([[
		INSERT INTO user_roles(
			role_name,
			user_id,
			target_team_slug
		) VALUES (
			$1,
			(SELECT id FROM users WHERE email = $2),
			$3
		)
	]], "Team owner", "authenticated@example.com", TeamSlug)

	DeleteKey = Helper.SQLQueryRow([[
		INSERT INTO team_delete_keys (
			team_slug,
			created_by
		) VALUES (
			$1,
			(SELECT id FROM users WHERE email = $2)
		) RETURNING key::TEXT;
	]], TeamSlug, "email-2@example.com")
end)

Test.gql("Request team deletion", function(t)
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
	]], TeamSlug))

	t.check {
		data = {
			requestTeamDeletion = {
				key = {
					key = Save("deleteKey")
				}
			}
		}
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
		team_slug = TeamSlug,
		email = "authenticated@example.com"
	}
end)
