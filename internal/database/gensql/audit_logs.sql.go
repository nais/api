// Code generated by sqlc. DO NOT EDIT.
// source: audit_logs.sql

package gensql

import (
	"context"

	"github.com/google/uuid"
)

const createAuditLog = `-- name: CreateAuditLog :exec
INSERT INTO audit_logs (correlation_id, actor, target_type, target_identifier, action, message)
VALUES ($1, $2, $3, $4, $5, $6)
`

type CreateAuditLogParams struct {
	CorrelationID    uuid.UUID
	Actor            *string
	TargetType       string
	TargetIdentifier string
	Action           string
	Message          string
}

func (q *Queries) CreateAuditLog(ctx context.Context, arg CreateAuditLogParams) error {
	_, err := q.db.Exec(ctx, createAuditLog,
		arg.CorrelationID,
		arg.Actor,
		arg.TargetType,
		arg.TargetIdentifier,
		arg.Action,
		arg.Message,
	)
	return err
}

const getAuditLogsForCorrelationID = `-- name: GetAuditLogsForCorrelationID :many
SELECT id, created_at, correlation_id, actor, action, message, target_type, target_identifier FROM audit_logs
WHERE correlation_id = $1
ORDER BY created_at DESC
LIMIT $3 OFFSET $2
`

type GetAuditLogsForCorrelationIDParams struct {
	CorrelationID uuid.UUID
	Offset        int32
	Limit         int32
}

func (q *Queries) GetAuditLogsForCorrelationID(ctx context.Context, arg GetAuditLogsForCorrelationIDParams) ([]*AuditLog, error) {
	rows, err := q.db.Query(ctx, getAuditLogsForCorrelationID, arg.CorrelationID, arg.Offset, arg.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []*AuditLog{}
	for rows.Next() {
		var i AuditLog
		if err := rows.Scan(
			&i.ID,
			&i.CreatedAt,
			&i.CorrelationID,
			&i.Actor,
			&i.Action,
			&i.Message,
			&i.TargetType,
			&i.TargetIdentifier,
		); err != nil {
			return nil, err
		}
		items = append(items, &i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getAuditLogsForCorrelationIDCount = `-- name: GetAuditLogsForCorrelationIDCount :one
SELECT COUNT(*) FROM audit_logs
WHERE correlation_id = $1
`

func (q *Queries) GetAuditLogsForCorrelationIDCount(ctx context.Context, correlationID uuid.UUID) (int64, error) {
	row := q.db.QueryRow(ctx, getAuditLogsForCorrelationIDCount, correlationID)
	var count int64
	err := row.Scan(&count)
	return count, err
}

const getAuditLogsForReconciler = `-- name: GetAuditLogsForReconciler :many
SELECT id, created_at, correlation_id, actor, action, message, target_type, target_identifier FROM audit_logs
WHERE target_type = 'reconciler' AND target_identifier = $1
ORDER BY created_at DESC
LIMIT $3 OFFSET $2
`

type GetAuditLogsForReconcilerParams struct {
	TargetIdentifier string
	Offset           int32
	Limit            int32
}

func (q *Queries) GetAuditLogsForReconciler(ctx context.Context, arg GetAuditLogsForReconcilerParams) ([]*AuditLog, error) {
	rows, err := q.db.Query(ctx, getAuditLogsForReconciler, arg.TargetIdentifier, arg.Offset, arg.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []*AuditLog{}
	for rows.Next() {
		var i AuditLog
		if err := rows.Scan(
			&i.ID,
			&i.CreatedAt,
			&i.CorrelationID,
			&i.Actor,
			&i.Action,
			&i.Message,
			&i.TargetType,
			&i.TargetIdentifier,
		); err != nil {
			return nil, err
		}
		items = append(items, &i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getAuditLogsForReconcilerCount = `-- name: GetAuditLogsForReconcilerCount :one
SELECT COUNT(*) FROM audit_logs
WHERE target_type = 'reconciler' AND target_identifier = $1
`

func (q *Queries) GetAuditLogsForReconcilerCount(ctx context.Context, targetIdentifier string) (int64, error) {
	row := q.db.QueryRow(ctx, getAuditLogsForReconcilerCount, targetIdentifier)
	var count int64
	err := row.Scan(&count)
	return count, err
}

const getAuditLogsForTeam = `-- name: GetAuditLogsForTeam :many
SELECT id, created_at, correlation_id, actor, action, message, target_type, target_identifier FROM audit_logs
WHERE target_type = 'team' AND target_identifier = $1
ORDER BY created_at DESC
LIMIT $3 OFFSET $2
`

type GetAuditLogsForTeamParams struct {
	TargetIdentifier string
	Offset           int32
	Limit            int32
}

func (q *Queries) GetAuditLogsForTeam(ctx context.Context, arg GetAuditLogsForTeamParams) ([]*AuditLog, error) {
	rows, err := q.db.Query(ctx, getAuditLogsForTeam, arg.TargetIdentifier, arg.Offset, arg.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []*AuditLog{}
	for rows.Next() {
		var i AuditLog
		if err := rows.Scan(
			&i.ID,
			&i.CreatedAt,
			&i.CorrelationID,
			&i.Actor,
			&i.Action,
			&i.Message,
			&i.TargetType,
			&i.TargetIdentifier,
		); err != nil {
			return nil, err
		}
		items = append(items, &i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getAuditLogsForTeamCount = `-- name: GetAuditLogsForTeamCount :one
SELECT COUNT(*) FROM audit_logs
WHERE target_type = 'team' AND target_identifier = $1
`

func (q *Queries) GetAuditLogsForTeamCount(ctx context.Context, targetIdentifier string) (int64, error) {
	row := q.db.QueryRow(ctx, getAuditLogsForTeamCount, targetIdentifier)
	var count int64
	err := row.Scan(&count)
	return count, err
}
