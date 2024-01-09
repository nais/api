-- name: GetReconcilerStateForTeam :one
SELECT * FROM reconciler_states
WHERE reconciler = @reconciler_name AND team_slug = @team_slug;

-- name: GetTeamsWithPermissionInGitHubRepo :many
SELECT t.* FROM teams t
JOIN reconciler_states rs ON rs.team_slug = t.slug
WHERE
    rs.reconciler = 'github:team'
    AND rs.state @> @state_matcher
ORDER BY t.slug ASC LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');


-- name: GetTeamsWithPermissionInGitHubRepoCount :one
SELECT count(1) FROM teams t
JOIN reconciler_states rs ON rs.team_slug = t.slug
WHERE
    rs.reconciler = 'github:team'
    AND rs.state @> @state_matcher
;

-- name: SetReconcilerStateForTeam :exec
INSERT INTO reconciler_states (reconciler, team_slug, state)
VALUES(@reconciler, @team_slug, @state)
ON CONFLICT (reconciler, team_slug) DO
    UPDATE SET state = @state;

-- name: RemoveReconcilerStateForTeam :exec
DELETE FROM reconciler_states
WHERE reconciler = @reconciler_name AND team_slug = @team_slug;
