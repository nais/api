-- name: GetAuditEventsForTeam :many
SELECT * FROM audit_events
WHERE team_slug = @team
ORDER BY created_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CreateAuditEvent :exec
INSERT INTO audit_events (actor, action, resource_type, resource_name, team_slug, data)
VALUES (@actor, @action, @resource_type, @resource_name, @team, @data);

-- name: GetAuditEventsCountForTeam :one
SELECT COUNT(*) FROM audit_events
WHERE team_slug = @team;
