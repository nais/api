-- name: CreateAuditLog :exec
INSERT INTO audit_logs (correlation_id, actor, component_name, target_type, target_identifier, action, message)
VALUES (@correlation_id, @actor, @component_name, @target_type, @target_identifier, @action, @message);

-- name: GetAuditLogsForTeam :many
SELECT * FROM audit_logs
WHERE target_type = 'team' AND target_identifier = @target_identifier
ORDER BY created_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetAuditLogsForTeamCount :one
SELECT COUNT(*) FROM audit_logs
WHERE target_type = 'team' AND target_identifier = @target_identifier;

-- name: GetAuditLogsForCorrelationID :many
SELECT * FROM audit_logs
WHERE correlation_id = @correlation_id
ORDER BY created_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetAuditLogsForCorrelationIDCount :one
select COUNT(*) from audit_logs
where correlation_id = @correlation_id;

-- name: GetAuditLogsForReconciler :many
SELECT * FROM audit_logs
WHERE target_type = 'reconciler' AND target_identifier = @target_identifier
ORDER BY created_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: GetAuditLogsForReconcilerCount :one
SELECT COUNT(*) FROM audit_logs
WHERE target_type = 'reconciler' AND target_identifier = @target_identifier;
