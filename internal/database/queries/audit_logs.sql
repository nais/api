-- name: CreateAuditLog :exec
INSERT INTO audit_logs (correlation_id, actor, component_name, target_type, target_identifier, action, message)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: GetAuditLogsForTeam :many
SELECT * FROM audit_logs
WHERE target_type = 'team' AND target_identifier = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetAuditLogsForTeamCount :one
SELECT COUNT(*) FROM audit_logs
WHERE target_type = 'team' AND target_identifier = $1;

-- name: GetAuditLogsForCorrelationID :many
SELECT * FROM audit_logs
WHERE correlation_id = $1
ORDER BY created_at DESC LIMIT $2 OFFSET $3;

-- name: GetAuditLogsForCorrelationIDCount :one
select COUNT(*) from audit_logs
where correlation_id = $1;

-- name: GetAuditLogsForReconciler :many
SELECT * FROM audit_logs
WHERE target_type = 'reconciler' AND target_identifier = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetAuditLogsForReconcilerCount :one
SELECT COUNT(*) FROM audit_logs
WHERE target_type = 'reconciler' AND target_identifier = $1;