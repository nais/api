-- name: SetLastSuccessfulSync :exec
UPDATE teams
SET
	last_successful_sync = NOW()
WHERE
	teams.slug = @slug
;

-- name: Delete :exec
DELETE FROM teams
WHERE
	slug = @slug
	AND delete_key_confirmed_at IS NOT NULL
;

-- name: Get :one
SELECT
	*
FROM
	teams
WHERE
	slug = @slug
;

-- name: List :many
SELECT
	*
FROM
	teams
ORDER BY
	slug ASC
LIMIT
	sqlc.arg('limit')
OFFSET
	sqlc.arg('offset')
;

-- name: Count :one
SELECT
	COUNT(*) AS total
FROM
	teams
;

-- name: ListMembers :many
SELECT
	users.*
FROM
	user_roles
	JOIN teams ON teams.slug = user_roles.target_team_slug
	JOIN users ON users.id = user_roles.user_id
WHERE
	user_roles.target_team_slug = @team_slug::slug
ORDER BY
	users.name ASC
LIMIT
	sqlc.arg('limit')
OFFSET
	sqlc.arg('offset')
;

-- name: CountMembers :one
SELECT
	COUNT(user_roles.*) AS total
FROM
	user_roles
	JOIN teams ON teams.slug = user_roles.target_team_slug
WHERE
	user_roles.target_team_slug = @team_slug::slug
;

-- name: UpdateExternalReferences :exec
UPDATE teams
SET
	google_group_email = COALESCE(@google_group_email, google_group_email),
	entra_id_group_id = COALESCE(@entra_id_group_id, entra_id_group_id),
	github_team_slug = COALESCE(@github_team_slug, github_team_slug),
	gar_repository = COALESCE(@gar_repository, gar_repository),
	cdn_bucket = COALESCE(@cdn_bucket, cdn_bucket)
WHERE
	teams.slug = @slug
;

-- name: UpsertEnvironment :exec
INSERT INTO
	team_environments (
		team_slug,
		environment,
		slack_alerts_channel,
		gcp_project_id
	)
VALUES
	(
		@team_slug,
		@environment,
		@slack_alerts_channel,
		@gcp_project_id
	)
ON CONFLICT (team_slug, environment) DO UPDATE
SET
	slack_alerts_channel = COALESCE(
		EXCLUDED.slack_alerts_channel,
		team_environments.slack_alerts_channel
	),
	gcp_project_id = COALESCE(
		EXCLUDED.gcp_project_id,
		team_environments.gcp_project_id
	)
;

-- name: ListEnvironments :many
SELECT
	team_all_environments.*
FROM
	team_all_environments
	JOIN teams ON teams.slug = team_all_environments.team_slug
WHERE
	team_all_environments.team_slug = @team_slug
ORDER BY
	team_all_environments.environment ASC
LIMIT
	sqlc.arg('limit')
OFFSET
	sqlc.arg('offset')
;

-- name: CountEnvironments :one
SELECT
	COUNT(team_all_environments.*) AS total
FROM
	team_all_environments
	JOIN teams ON teams.slug = team_all_environments.team_slug
WHERE
	team_all_environments.team_slug = @team_slug
;
