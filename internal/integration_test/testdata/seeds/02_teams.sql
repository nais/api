-- Create a bunch of teams
INSERT INTO teams (
  slug,
  purpose,
  slack_channel
)
SELECT
  concat('slug-', generate_series),
  concat('purpose-', generate_series),
  concat('#slack_channel-', generate_series)
FROM generate_series(1, 20);


-- Join users to teams
INSERT INTO user_roles (
  role_name,
  user_id,
  target_team_slug
)
SELECT
  (ARRAY['Team owner'::role_name,'Team member'::role_name])[round(random())+1],
  u.id,
  t.slug
FROM users u
JOIN teams t ON random() < 0.5;
