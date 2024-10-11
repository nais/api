TeamSlug = "delete-me"

Test.sql("Initialize team", function(t)
	Helper.SQLExec([[
		INSERT INTO teams(
			slug,
			purpose,
			slack_channel
		) VALUES (
			$1,
			$2,
			$3
		)
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

Test.gql("Confirm team deletion", function(t)
	t.query(string.format([[
		mutation {
			confirmTeamDeletion(input: {
				slug: "%s"
				key: "%s"
			}) {
				deletionStarted
			}
		}
	]], TeamSlug, DeleteKey.key))

	t.check {
		data = {
			confirmTeamDeletion = {
				deletionStarted = true
			}
		}
	}
end)

Test.pubsub("Team deleted event", function(t)
	t.check("topic", {
		attributes = {
			CorrelationID = NotNull(),
			EventType = "EVENT_TEAM_DELETED"
		},
		data = {
			slug = TeamSlug
		}
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
	]], DeleteKey.key)

	t.check {
		team_slug = TeamSlug
	}
end)