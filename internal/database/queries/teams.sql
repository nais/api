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
SELECT team_environments.*
FROM team_environments
WHERE team_environments.team_slug = @team_slug
ORDER BY team_environments.environment ASC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetTeamEnvironmentsCount :one
SELECT COUNT(*) as total
FROM team_environments
WHERE team_slug = @team_slug;

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
WHERE user_roles.target_team_slug = @team_slug
ORDER BY users.name ASC LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetTeamMembersCount :one
SELECT COUNT (*) FROM user_roles
WHERE user_roles.target_team_slug = @team_slug;

-- name: GetTeamMember :one
SELECT users.* FROM user_roles
JOIN teams ON teams.slug = user_roles.target_team_slug
JOIN users ON users.id = user_roles.user_id
WHERE user_roles.target_team_slug = @team_slug AND users.id = @user_id
ORDER BY users.name ASC;

-- name: GetTeamMembersForReconciler :many
SELECT users.* FROM user_roles
JOIN teams ON teams.slug = user_roles.target_team_slug
JOIN users ON users.id = user_roles.user_id
WHERE
    user_roles.target_team_slug = @team_slug
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

-- name: SetGoogleGroupEmailForTeam :exec
UPDATE teams
SET google_group_email = @google_group_email::text
WHERE slug = @slug;

-- name: RemoveUserFromTeam :exec
DELETE FROM user_roles
WHERE user_id = @user_id AND target_team_slug = @team_slug;

-- name: SetLastSuccessfulSyncForTeam :exec
UPDATE teams SET last_successful_sync = NOW()
WHERE slug = @slug;

-- name: GetSlackAlertsChannels :many
SELECT * FROM slack_alerts_channels
WHERE team_slug = @team_slug
ORDER BY environment ASC;

-- name: SetSlackAlertsChannel :exec
INSERT INTO slack_alerts_channels (team_slug, environment, channel_name)
VALUES (@team_slug, @environment, @channel_name)
ON CONFLICT (team_slug, environment) DO
    UPDATE SET channel_name = @channel_name;

-- name: RemoveSlackAlertsChannel :exec
DELETE FROM slack_alerts_channels
WHERE team_slug = @team_slug AND environment = @environment;

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
WHERE slug = @slug;

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

-- name: SearchTeams :many
SELECT *
FROM teams
WHERE levenshtein(@slug_match::text, slug) >= 0
ORDER BY levenshtein(@slug_match::text, slug) ASC
LIMIT sqlc.arg('limit');
;

-- name: TeamExists :one
SELECT EXISTS(
    SELECT 1 FROM teams
    WHERE slug = @slug
) AS exists;
