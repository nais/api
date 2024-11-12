-- name: Create :one
INSERT INTO
	teams (slug, purpose, slack_channel)
VALUES
	(@slug, @purpose, @slack_channel)
RETURNING
	*
;

-- name: Update :one
UPDATE teams
SET
	purpose = COALESCE(sqlc.narg(purpose), purpose),
	slack_channel = COALESCE(sqlc.narg(slack_channel), slack_channel)
WHERE
	teams.slug = @slug
RETURNING
	*
;

-- name: Exists :one
SELECT
	EXISTS (
		SELECT
			slug
		FROM
			teams
		WHERE
			slug = @slug
	)
;

-- name: SlugAvailable :one
SELECT
	NOT EXISTS (
		SELECT
			slug
		FROM
			team_slugs
		WHERE
			slug = @slug
	)
;

-- name: Count :one
SELECT
	COUNT(*)
FROM
	teams
;

-- name: List :many
SELECT
	*
FROM
	teams
ORDER BY
	CASE
		WHEN @order_by::TEXT = 'slug:asc' THEN slug
	END ASC,
	CASE
		WHEN @order_by::TEXT = 'slug:desc' THEN slug
	END DESC,
	slug ASC
LIMIT
	sqlc.arg('limit')
OFFSET
	sqlc.arg('offset')
;

-- name: Get :one
SELECT
	*
FROM
	teams
WHERE
	slug = @slug
;

-- name: ListBySlugs :many
SELECT
	*
FROM
	teams
WHERE
	slug = ANY (@slugs::slug[])
ORDER BY
	slug ASC
;

-- ListEnvironmentsBySlugsAndEnvNames returns a slice of team environments for a list of teams/envs, excluding
-- deleted teams.
-- name: ListEnvironmentsBySlugsAndEnvNames :many
-- Input is two arrays of equal length, one for slugs and one for names
WITH
	input AS (
		SELECT
			UNNEST(@team_slugs::slug[]) AS team_slug,
			UNNEST(@environments::TEXT[]) AS environment
	)
SELECT
	team_all_environments.*
FROM
	team_all_environments
	JOIN input ON input.team_slug = team_all_environments.team_slug
	JOIN teams ON teams.slug = team_all_environments.team_slug
WHERE
	team_all_environments.environment = input.environment
ORDER BY
	team_all_environments.environment ASC
;

-- name: ListEnvironmentsBySlug :many
SELECT
	*
FROM
	team_all_environments
WHERE
	team_slug = @slug
ORDER BY
	environment ASC
;

-- name: GetEnvironment :one
SELECT
	*
FROM
	team_all_environments
WHERE
	team_slug = @slug
	AND environment = @environment
;

-- name: CreateDeleteKey :one
INSERT INTO
	team_delete_keys (team_slug, created_by)
VALUES
	(@team_slug, @created_by)
RETURNING
	*
;

-- name: GetDeleteKey :one
SELECT
	*
FROM
	team_delete_keys
WHERE
	key = @key
	AND team_slug = @slug::slug
;

-- name: ConfirmDeleteKey :exec
UPDATE team_delete_keys
SET
	confirmed_at = NOW()
WHERE
	key = @key
;

-- name: SetDeleteKeyConfirmedAt :exec
UPDATE teams
SET
	delete_key_confirmed_at = NOW()
WHERE
	slug = @slug
;

-- name: Search :many
WITH
	result AS (
		SELECT
			slug,
			levenshtein (@query, slug) AS RANK
		FROM
			teams
		ORDER BY
			RANK ASC
		LIMIT
			10
	)
SELECT
	sqlc.embed(teams),
	RANK
FROM
	teams
	JOIN result ON teams.slug = result.slug
ORDER BY
	result.rank ASC
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

-- name: RemoveSlackAlertsChannel :exec
UPDATE team_environments
SET
	slack_alerts_channel = NULL
WHERE
	team_slug = @team_slug
	AND environment = @environment
;

-- name: UpdateExternalReferences :exec
UPDATE teams
SET
	google_group_email = COALESCE(@google_group_email, google_group_email),
	azure_group_id = COALESCE(@entra_id_group_id, azure_group_id),
	github_team_slug = COALESCE(@github_team_slug, github_team_slug),
	gar_repository = COALESCE(@gar_repository, gar_repository),
	cdn_bucket = COALESCE(@cdn_bucket, cdn_bucket)
WHERE
	teams.slug = @slug
;

-- name: ListAllSlugs :one
SELECT
	ARRAY_AGG(slug)::slug[] AS slugs
FROM
	teams
;
