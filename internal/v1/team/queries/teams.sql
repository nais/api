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

-- name: SlugAvailable :one
SELECT
	EXISTS (
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

-- ListEnvironmentsBySlug
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
