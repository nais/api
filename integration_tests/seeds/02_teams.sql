-- Create a bunch of teams
INSERT INTO
	teams (slug, purpose, slack_channel)
SELECT
	CONCAT('slug-', GENERATE_SERIES),
	CONCAT('purpose-', GENERATE_SERIES),
	CONCAT('#slack_channel-', GENERATE_SERIES)
FROM
	GENERATE_SERIES(1, 20)
;

-- Join users to teams
INSERT INTO
	user_roles (role_name, user_id, target_team_slug)
SELECT
	(ARRAY['Team owner', 'Team member']) [ROUND(RANDOM()) + 1],
	u.id,
	t.slug
FROM
	users u
	JOIN teams t ON RANDOM() < 0.5
;
