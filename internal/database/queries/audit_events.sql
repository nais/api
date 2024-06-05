-- name: GetAuditEventsForTeam :many
SELECT * FROM audit_events
WHERE team_slug = @team
ORDER BY created_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CreateAuditEvent :one
INSERT INTO audit_events (actor, action, resource_type, resource_name, team_slug)
VALUES (@actor, @action, @resource_type, @resource_name, @team)
RETURNING id;

-- name: CreateAuditEventData :exec
INSERT INTO audit_events_data (event_id, key, value)
VALUES (@event_id, @key, @value);
