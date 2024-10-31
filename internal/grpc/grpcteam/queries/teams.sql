-- name: SetLastSuccessfulSyncForTeam :exec
UPDATE teams
SET
	last_successful_sync = NOW()
WHERE
	teams.slug = @slug
;

-- name: DeleteTeam :exec
DELETE FROM teams
WHERE
	slug = @slug
	AND delete_key_confirmed_at IS NOT NULL
;

-- name: GetTeamBySlug :one
SELECT
	*
FROM
	teams
WHERE
	slug = @slug
;

-- name: GetTeams :many
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

-- name: GetTeamsCount :one
SELECT
	COUNT(*) AS total
FROM
	teams
;

-- name: GetTeamMembers :many
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

-- name: GetTeamMembersCount :one
SELECT
	COUNT(user_roles.*) AS total
FROM
	user_roles
	JOIN teams ON teams.slug = user_roles.target_team_slug
WHERE
	user_roles.target_team_slug = @team_slug::slug
;

-- name: UpdateTeamExternalReferences :exec
UPDATE teams
SET
	google_group_email = COALESCE(@google_group_email, google_group_email),
	azure_group_id = COALESCE(@azure_group_id, azure_group_id),
	github_team_slug = COALESCE(@github_team_slug, github_team_slug),
	gar_repository = COALESCE(@gar_repository, gar_repository),
	cdn_bucket = COALESCE(@cdn_bucket, cdn_bucket)
WHERE
	teams.slug = @slug
;

-- name: UpsertTeamEnvironment :exec
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
ON CONFLICT (team_slug, environment) DO
UPDATE
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

-- name: GetTeamEnvironments :many
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

-- name: GetTeamEnvironmentsCount :one
SELECT
	COUNT(team_all_environments.*) AS total
FROM
	team_all_environments
	JOIN teams ON teams.slug = team_all_environments.team_slug
WHERE
	team_all_environments.team_slug = @team_slug
;
