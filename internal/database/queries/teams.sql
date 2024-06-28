-- CreateTeam creates a new team.
-- name: CreateTeam :one
INSERT INTO teams (slug, purpose, slack_channel)
VALUES (@slug, @purpose, @slack_channel)
RETURNING *;

-- GetTeamEnvironments returns a slice of team environments, excluding deleted teams.
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

-- GetTeamEnvironmentsCount returns the total number of team environments, excluding deleted teams.
-- name: GetTeamEnvironmentsCount :one
SELECT COUNT(team_all_environments.*) AS total
FROM team_all_environments
JOIN teams ON teams.slug = team_all_environments.team_slug
WHERE
    team_all_environments.team_slug = @team_slug
    AND teams.deleted_at IS NULL;

-- GetTeamEnvironmentsBySlugsAndEnvNames returns a slice of team environments for a list of teams/envs, excluding
-- deleted teams.
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

-- UpsertTeamEnvironment creates or updates a team environment.
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
SELECT teams.*
FROM teams
WHERE teams.deleted_at IS NULL
ORDER BY teams.slug ASC
LIMIT sqlc.arg('limit')
OFFSET sqlc.arg('offset');

-- GetTeamsCount returns the total number or teams, excluding deleted teams.
-- name: GetTeamsCount :one
SELECT COUNT(teams.*) AS total
FROM teams
WHERE teams.deleted_at IS NULL;

-- GetActiveTeams returns a slice of teams that can be reconciled.
-- name: GetActiveTeams :many
SELECT teams.*
FROM teams
WHERE
    teams.delete_key_confirmed_at IS NULL
    AND teams.deleted_at IS NULL
ORDER BY teams.slug ASC
LIMIT sqlc.arg('limit')
OFFSET sqlc.arg('offset');

-- GetActiveTeamsCount returns the total number or teams that can be reconciled.
-- name: GetActiveTeamsCount :one
SELECT COUNT(teams.*) AS total
FROM teams
WHERE
    teams.delete_key_confirmed_at IS NULL
    AND teams.deleted_at IS NULL;

-- GetDeletableTeams returns a slice of teams that is ready to start the deletion process.
-- name: GetDeletableTeams :many
SELECT teams.*
FROM teams
WHERE
    teams.delete_key_confirmed_at IS NOT NULL
    AND teams.deleted_at IS NULL
ORDER BY teams.slug ASC
LIMIT sqlc.arg('limit')
OFFSET sqlc.arg('offset');

-- GetDeletableTeamsCount returns the total number or teams that is ready to start the deletion process.
-- name: GetDeletableTeamsCount :one
SELECT COUNT(teams.*) AS total
FROM teams
WHERE
    teams.delete_key_confirmed_at IS NOT NULL
    AND teams.deleted_at IS NULL;

-- GetAllTeamSlugs returns all team slugs in ascending order, excluding deleted teams.
-- name: GetAllTeamSlugs :many
SELECT teams.slug
FROM teams
WHERE teams.deleted_at IS NULL
ORDER BY teams.slug ASC;

-- GetTeamBySlug returns a team by its slug, excluding deleted teams.
-- name: GetTeamBySlug :one
SELECT teams.*
FROM teams
WHERE
    teams.slug = @slug
    AND teams.deleted_at IS NULL;

-- GetTeamsBySlugs returns a slice of teams by their slugs, excluding deleted teams.
-- name: GetTeamsBySlugs :many
SELECT teams.*
FROM teams
WHERE
    teams.slug = ANY(@slugs::slug[])
    AND teams.deleted_at IS NULL
ORDER BY teams.slug ASC;

-- GetAllTeamMembers returns all team members of a non-deleted team.
-- name: GetAllTeamMembers :many
SELECT users.*
FROM user_roles
JOIN teams ON teams.slug = user_roles.target_team_slug
JOIN users ON users.id = user_roles.user_id
WHERE
    user_roles.target_team_slug = @team_slug
    AND teams.deleted_at IS NULL
ORDER BY users.name ASC;

-- GetTeamMembers returns a slice of team members of a non-deleted team.
-- name: GetTeamMembers :many
SELECT users.*
FROM user_roles
JOIN teams ON teams.slug = user_roles.target_team_slug
JOIN users ON users.id = user_roles.user_id
WHERE
    user_roles.target_team_slug = @team_slug::slug
    AND teams.deleted_at IS NULL
ORDER BY users.name ASC
LIMIT sqlc.arg('limit')
OFFSET sqlc.arg('offset');

-- GetTeamMembersCount returns the total number of team members of a non-deleted team.
-- name: GetTeamMembersCount :one
SELECT COUNT(user_roles.*) AS total
FROM user_roles
JOIN teams ON teams.slug = user_roles.target_team_slug
WHERE
    user_roles.target_team_slug = @team_slug
    AND teams.deleted_at IS NULL;

-- GetTeamMember returns a specific team member of a non-deleted team.
-- name: GetTeamMember :one
SELECT users.*
FROM user_roles
JOIN teams ON teams.slug = user_roles.target_team_slug
JOIN users ON users.id = user_roles.user_id
WHERE
    user_roles.target_team_slug = @team_slug::slug
    AND users.id = @user_id
    AND teams.deleted_at IS NULL
ORDER BY users.name ASC;

-- UpdateTeam updates the purpose and slack channel of a non-deleted team.
-- name: UpdateTeam :one
UPDATE teams
SET
    purpose = COALESCE(sqlc.narg(purpose), purpose),
    slack_channel = COALESCE(sqlc.narg(slack_channel), slack_channel)
WHERE
    teams.slug = @slug
    AND teams.deleted_at IS NULL
RETURNING *;

-- UpdateTeamExternalReferences updates the external references of a non-deleted team.
-- name: UpdateTeamExternalReferences :one
UPDATE teams
SET
    google_group_email = COALESCE(@google_group_email, google_group_email),
    azure_group_id =  COALESCE(@azure_group_id, azure_group_id),
    github_team_slug = COALESCE(@github_team_slug, github_team_slug),
    gar_repository = COALESCE(@gar_repository, gar_repository),
    cdn_bucket = COALESCE(@cdn_bucket, cdn_bucket)
WHERE
    teams.slug = @slug
    AND teams.deleted_at IS NULL
RETURNING *;

-- RemoveUserFromTeam removes a user from a team.
-- name: RemoveUserFromTeam :exec
DELETE FROM user_roles
WHERE
    user_roles.user_id = @user_id
    AND user_roles.target_team_slug = @team_slug::slug;

-- SetLastSuccessfulSyncForTeam sets the last successful sync time for a non-deleted team.
-- name: SetLastSuccessfulSyncForTeam :exec
UPDATE teams
SET teams.last_successful_sync = NOW()
WHERE
    teams.slug = @slug
    AND teams.deleted_at IS NULL;

-- CreateTeamDeleteKey creates a new delete key for a team.
-- name: CreateTeamDeleteKey :one
INSERT INTO team_delete_keys (team_slug, created_by)
VALUES(@team_slug, @created_by)
RETURNING *;

-- GetTeamDeleteKey returns a delete key for a team.
-- name: GetTeamDeleteKey :one
SELECT team_delete_keys.*
FROM team_delete_keys
WHERE team_delete_keys.key = @key;

-- ConfirmTeamDeleteKey confirms a delete key for a team.
-- name: ConfirmTeamDeleteKey :exec
UPDATE team_delete_keys
SET team_delete_keys.confirmed_at = NOW()
WHERE team_delete_keys.key = @key;

-- DeleteTeam marks a team as deleted. The team must have an already confirmed delete key for this to succeed.
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

-- GetTeamMemberOptOuts returns a slice of team member opt-outs.
-- name: GetTeamMemberOptOuts :many
SELECT
    reconcilers.name,
    NOT EXISTS(
        SELECT reconciler_name
        FROM reconciler_opt_outs
        WHERE
            reconciler_opt_outs.user_id = @user_id
            AND reconciler_opt_outs.team_slug = @team_slug
            AND reconciler_opt_outs.reconciler_name = reconcilers.name
    ) AS enabled
FROM reconcilers
WHERE reconcilers.enabled = true
ORDER BY reconcilers.name ASC;

-- TeamExists checks if a team exists. Deleted teams are not considered.
-- name: TeamExists :one
SELECT EXISTS(
    SELECT 1 FROM teams
    WHERE
        teams.slug = @slug
        AND teams.deleted_at IS NULL
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

-- name: SetTeamDeleteKeyConfirmedAt :exec
UPDATE teams
SET delete_key_confirmed_at = NOW()
WHERE slug = @slug;