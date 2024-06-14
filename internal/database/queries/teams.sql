-- name: CreateTeam :one
INSERT INTO teams (slug, purpose, slack_channel)
VALUES (@slug, @purpose, @slack_channel)
RETURNING *;

-- name: GetActiveTeams :many
SELECT teams.* FROM teams
WHERE NOT EXISTS (
    SELECT team_delete_keys.team_slug
    FROM team_delete_keys
    WHERE
        team_delete_keys.team_slug = teams.slug
        AND team_delete_keys.confirmed_at IS NOT NULL
)
ORDER BY teams.slug ASC;

-- name: GetTeamEnvironments :many
SELECT *
FROM team_all_environments
WHERE team_slug = @team_slug
ORDER BY environment ASC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetTeamEnvironmentsCount :one
SELECT COUNT(*) as total
FROM team_all_environments
WHERE team_slug = @team_slug;

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
WHERE team_all_environments.environment = input.environment
ORDER BY team_all_environments.environment ASC;
;

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

-- name: GetAllTeamSlugs :many
SELECT teams.slug FROM teams
ORDER BY teams.slug ASC;

-- name: GetTeams :many
SELECT teams.* FROM teams
ORDER BY teams.slug ASC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetTeamsCount :one
SELECT COUNT(*) as total FROM teams;

-- name: GetActiveTeamBySlug :one
SELECT teams.* FROM teams
WHERE
    teams.slug = @slug
    AND NOT EXISTS (
        SELECT team_delete_keys.team_slug
        FROM team_delete_keys
        WHERE
            team_delete_keys.team_slug = @slug
            AND team_delete_keys.confirmed_at IS NOT NULL
    );

-- name: GetTeamBySlug :one
SELECT teams.* FROM teams
WHERE teams.slug = @slug;

-- name: GetTeamBySlugs :many
SELECT * FROM teams
WHERE slug = ANY(@slugs::slug[])
ORDER BY slug ASC;

-- name: GetAllTeamMembers :many
SELECT users.* FROM user_roles
JOIN teams ON teams.slug = user_roles.target_team_slug
JOIN users ON users.id = user_roles.user_id
WHERE user_roles.target_team_slug = @team_slug
ORDER BY users.name ASC;

-- name: GetTeamMembers :many
SELECT users.* FROM user_roles
JOIN teams ON teams.slug = user_roles.target_team_slug
JOIN users ON users.id = user_roles.user_id
WHERE user_roles.target_team_slug = @team_slug::slug
ORDER BY users.name ASC LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetTeamMembersCount :one
SELECT COUNT(*) FROM user_roles
WHERE user_roles.target_team_slug = @team_slug;

-- name: GetTeamMember :one
SELECT users.* FROM user_roles
JOIN teams ON teams.slug = user_roles.target_team_slug
JOIN users ON users.id = user_roles.user_id
WHERE user_roles.target_team_slug = @team_slug::slug AND users.id = @user_id
ORDER BY users.name ASC;

-- name: GetTeamMembersForReconciler :many
SELECT users.* FROM user_roles
JOIN teams ON teams.slug = user_roles.target_team_slug
JOIN users ON users.id = user_roles.user_id
WHERE
    user_roles.target_team_slug = @team_slug::slug
    AND NOT EXISTS (
        SELECT roo.user_id
        FROM reconciler_opt_outs AS roo
        WHERE
            roo.team_slug = @team_slug
            AND roo.reconciler_name = @reconciler_name
            AND roo.user_id = users.id
    )
ORDER BY users.name ASC;

-- name: UpdateTeam :one
UPDATE teams
SET purpose = COALESCE(sqlc.narg(purpose), purpose),
    slack_channel = COALESCE(sqlc.narg(slack_channel), slack_channel)
WHERE slug = @slug
RETURNING *;

-- name: UpdateTeamExternalReferences :one
UPDATE teams
SET google_group_email = COALESCE(@google_group_email, google_group_email),
    azure_group_id =  COALESCE(@azure_group_id, azure_group_id),
    github_team_slug = COALESCE(@github_team_slug, github_team_slug),
    gar_repository = COALESCE(@gar_repository, gar_repository),
    cdn_bucket = COALESCE(@cdn_bucket, cdn_bucket)
WHERE slug = @slug
RETURNING *;

-- name: RemoveUserFromTeam :exec
DELETE FROM user_roles
WHERE user_id = @user_id AND target_team_slug = @team_slug::slug;

-- name: SetLastSuccessfulSyncForTeam :exec
UPDATE teams SET last_successful_sync = NOW()
WHERE slug = @slug;

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

-- name: DeleteTeam :exec
DELETE FROM teams
WHERE
    teams.slug = @slug
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

-- name: TeamExists :one
SELECT EXISTS(
    SELECT 1 FROM teams
    WHERE slug = @slug
) AS exists;
