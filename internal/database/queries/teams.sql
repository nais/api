-- name: CreateTeam :one
INSERT INTO teams (slug, purpose, slack_channel)
VALUES (@slug, @purpose, @slack_channel)
RETURNING *;

-- name: GetTeamEnvironments :many
SELECT team_all_environments.*
FROM team_all_environments
JOIN teams ON teams.slug = team_all_environments.team_slug
WHERE
    team_all_environments.team_slug = @team_slug
    AND teams.deleted_at IS NULL
ORDER BY team_all_environments.environment ASC
LIMIT sqlc.arg('limit')
OFFSET sqlc.arg('offset');

-- name: GetTeamEnvironmentsCount :one
SELECT COUNT(tae.*) as total
FROM team_all_environments tae
JOIN teams ON teams.slug = tae.team_slug
WHERE
    tae.team_slug = @team_slug
    AND teams.deleted_at IS NULL;

-- name: GetTeamEnvironmentsBySlugsAndEnvNames :many
-- Input is two arrays of equal length, one for slugs and one for names
WITH input AS (
    SELECT
        unnest(@team_slugs::slug[]) AS team_slug,
        unnest(@environments::text[]) AS environment
)
SELECT team_all_environments.*
FROM team_all_environments
JOIN input ON input.team_slug = team_all_environments.team_slug
JOIN teams ON teams.slug = team_all_environments.team_slug
WHERE
    team_all_environments.environment = input.environment
    AND teams.deleted_at IS NULL
ORDER BY team_all_environments.environment ASC;

-- name: UpsertTeamEnvironment :one
INSERT INTO team_environments (team_slug, environment, slack_alerts_channel, gcp_project_id)
VALUES (
    @team_slug,
    @environment,
    @slack_alerts_channel,
    @gcp_project_id
)
ON CONFLICT (team_slug, environment) DO UPDATE
SET
    slack_alerts_channel = COALESCE(EXCLUDED.slack_alerts_channel, team_environments.slack_alerts_channel),
    gcp_project_id = COALESCE(EXCLUDED.gcp_project_id, team_environments.gcp_project_id)
RETURNING *;

-- GetTeams returns a slice of teams, excluding deleted teams.
-- name: GetTeams :many
SELECT teams.* FROM teams
WHERE teams.deleted_at IS NULL
ORDER BY teams.slug ASC
LIMIT sqlc.arg('limit')
OFFSET sqlc.arg('offset');

-- GetTeamsCount returns the total number or teams, excluding deleted teams.
-- name: GetTeamsCount :one
SELECT COUNT(*) as total
FROM teams
WHERE teams.deleted_at IS NULL;

-- GetActiveOrDeletedTeams returns a slice of teams, including deleted teams.
-- name: GetActiveOrDeletedTeams :many
SELECT teams.* FROM teams
ORDER BY teams.slug ASC
LIMIT sqlc.arg('limit')
OFFSET sqlc.arg('offset');

-- GetActiveOrDeletedTeamsCount returns the total number or teams, including deleted teams.
-- name: GetActiveOrDeletedTeamsCount :one
SELECT COUNT(*) as total
FROM teams;

-- GetAllTeamSlugs returns all team slugs in ascending order, excluding deleted teams.
-- name: GetAllTeamSlugs :many
SELECT teams.slug
FROM teams
WHERE teams.deleted_at IS NULL
ORDER BY teams.slug ASC;

-- name: GetTeamBySlug :one
SELECT teams.* FROM teams
WHERE
    teams.slug = @slug
    AND teams.deleted_at IS NULL;

-- name: GetActiveOrDeletedTeamBySlug :one
SELECT teams.* FROM teams
WHERE teams.slug = @slug;

-- name: GetTeamsBySlugs :many
SELECT teams.* FROM teams
WHERE
    teams.slug = ANY(@slugs::slug[])
    AND teams.deleted_at IS NULL
ORDER BY teams.slug ASC;

-- name: GetAllTeamMembers :many
SELECT users.* FROM user_roles
JOIN teams ON teams.slug = user_roles.target_team_slug
JOIN users ON users.id = user_roles.user_id
WHERE
    user_roles.target_team_slug = @team_slug
    AND teams.deleted_at IS NULL
ORDER BY users.name ASC;

-- name: GetTeamMembers :many
SELECT users.* FROM user_roles
JOIN teams ON teams.slug = user_roles.target_team_slug
JOIN users ON users.id = user_roles.user_id
WHERE
    user_roles.target_team_slug = @team_slug::slug
    AND teams.deleted_at IS NULL
ORDER BY users.name ASC
LIMIT sqlc.arg('limit')
OFFSET sqlc.arg('offset');

-- name: GetTeamMembersCount :one
SELECT COUNT(*) FROM user_roles
JOIN teams ON teams.slug = user_roles.target_team_slug
WHERE
    user_roles.target_team_slug = @team_slug
    AND teams.deleted_at IS NULL;

-- name: GetTeamMember :one
SELECT users.* FROM user_roles
JOIN teams ON teams.slug = user_roles.target_team_slug
JOIN users ON users.id = user_roles.user_id
WHERE
    user_roles.target_team_slug = @team_slug::slug
    AND users.id = @user_id
    AND teams.deleted_at IS NULL
ORDER BY users.name ASC;

-- UpdateTeam updates the purpose and slack channel of a team when specified.
-- name: UpdateTeam :one
UPDATE teams
SET purpose = COALESCE(sqlc.narg(purpose), purpose),
    slack_channel = COALESCE(sqlc.narg(slack_channel), slack_channel)
WHERE
    slug = @slug
    AND deleted_at IS NULL
RETURNING *;

-- name: UpdateTeamExternalReferences :one
UPDATE teams
SET google_group_email = COALESCE(@google_group_email, google_group_email),
    azure_group_id =  COALESCE(@azure_group_id, azure_group_id),
    github_team_slug = COALESCE(@github_team_slug, github_team_slug),
    gar_repository = COALESCE(@gar_repository, gar_repository),
    cdn_bucket = COALESCE(@cdn_bucket, cdn_bucket)
WHERE
    slug = @slug
    AND deleted_at IS NULL
RETURNING *;

-- name: RemoveUserFromTeam :exec
DELETE FROM user_roles
WHERE user_id = @user_id AND target_team_slug = @team_slug::slug;

-- name: SetLastSuccessfulSyncForTeam :exec
UPDATE teams
SET last_successful_sync = NOW()
WHERE
    slug = @slug
    AND deleted_at IS NULL;

-- name: CreateTeamDeleteKey :one
INSERT INTO team_delete_keys (team_slug, created_by)
VALUES(@team_slug, @created_by)
RETURNING *;

-- name: GetTeamDeleteKey :one
SELECT * FROM team_delete_keys
WHERE key = @key;

-- name: ConfirmTeamDeleteKey :exec
UPDATE team_delete_keys
SET confirmed_at = NOW()
WHERE key = @key;

-- DeleteTeam marks a team as deleted. The team must have an already confirmed delete key for a successful deletion.
-- name: DeleteTeam :exec
UPDATE teams
SET teams.deleted_at = NOW()
WHERE
    teams.slug = @slug
    AND teams.deleted_at IS NULL
    AND EXISTS(
        SELECT team_delete_keys.team_slug
        FROM team_delete_keys
        WHERE
            team_delete_keys.team_slug = @slug
            AND team_delete_keys.confirmed_at IS NOT NULL
    );

-- name: GetTeamMemberOptOuts :many
SELECT
    reconcilers.name,
    NOT EXISTS(
        SELECT reconciler_name FROM reconciler_opt_outs
        WHERE user_id = @user_id AND team_slug = @team_slug AND reconciler_name = reconcilers.name
    ) AS enabled
FROM reconcilers
WHERE reconcilers.enabled = true
ORDER BY reconcilers.name ASC;

-- TeamExists checks if a team exists. Deleted teams are not considered.
-- name: TeamExists :one
SELECT EXISTS(
    SELECT 1 FROM teams
    WHERE
        slug = @slug
        AND deleted_at IS NULL
) AS exists;

-- TeamHasConfirmedDeleteKey checks if a team has a confirmed delete key. This means that the team is currently being
-- deleted. Already deleted teams are not considered.
-- name: TeamHasConfirmedDeleteKey :one
SELECT EXISTS(
    SELECT team_delete_keys.team_slug
    FROM team_delete_keys
    JOIN teams ON teams.slug = team_delete_keys.team_slug
    WHERE
        team_delete_keys.team_slug = @slug
        AND team_delete_keys.confirmed_at IS NOT NULL
        AND teams.deleted_at IS NULL
) AS exists;
