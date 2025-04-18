// Code generated by sqlc. DO NOT EDIT.
// source: deployments.sql

package deploymentsql

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/api/internal/slug"
)

const cleanupNaisVerification = `-- name: CleanupNaisVerification :execresult
DELETE FROM deployments
WHERE
	team_slug = 'nais-verification'
	AND created_at < NOW() - '1 week'::INTERVAL
`

func (q *Queries) CleanupNaisVerification(ctx context.Context) (pgconn.CommandTag, error) {
	return q.db.Exec(ctx, cleanupNaisVerification)
}

const latestDeploymentTimestampForWorkload = `-- name: LatestDeploymentTimestampForWorkload :one
SELECT
	deployments.created_at
FROM
	deployments
	JOIN deployment_k8s_resources ON deployments.id = deployment_k8s_resources.deployment_id
WHERE
	deployment_k8s_resources.name = $1
	AND deployment_k8s_resources.kind = $2
	AND deployments.environment_name = $3
	AND deployments.team_slug = $4
ORDER BY
	deployments.created_at DESC
LIMIT
	1
`

type LatestDeploymentTimestampForWorkloadParams struct {
	WorkloadName    string
	WorkloadKind    string
	EnvironmentName string
	TeamSlug        slug.Slug
}

func (q *Queries) LatestDeploymentTimestampForWorkload(ctx context.Context, arg LatestDeploymentTimestampForWorkloadParams) (pgtype.Timestamptz, error) {
	row := q.db.QueryRow(ctx, latestDeploymentTimestampForWorkload,
		arg.WorkloadName,
		arg.WorkloadKind,
		arg.EnvironmentName,
		arg.TeamSlug,
	)
	var created_at pgtype.Timestamptz
	err := row.Scan(&created_at)
	return created_at, err
}

const listByIDs = `-- name: ListByIDs :many
SELECT
	id, external_id, created_at, team_slug, repository, commit_sha, deployer_username, trigger_url, environment_name
FROM
	deployments
WHERE
	id = ANY ($1::UUID[])
ORDER BY
	id
`

func (q *Queries) ListByIDs(ctx context.Context, ids []uuid.UUID) ([]*Deployment, error) {
	rows, err := q.db.Query(ctx, listByIDs, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []*Deployment{}
	for rows.Next() {
		var i Deployment
		if err := rows.Scan(
			&i.ID,
			&i.ExternalID,
			&i.CreatedAt,
			&i.TeamSlug,
			&i.Repository,
			&i.CommitSha,
			&i.DeployerUsername,
			&i.TriggerUrl,
			&i.EnvironmentName,
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

const listByTeamSlug = `-- name: ListByTeamSlug :many
SELECT
	deployments.id, deployments.external_id, deployments.created_at, deployments.team_slug, deployments.repository, deployments.commit_sha, deployments.deployer_username, deployments.trigger_url, deployments.environment_name,
	COUNT(*) OVER () AS total_count
FROM
	deployments
WHERE
	team_slug = $1::slug
ORDER BY
	created_at DESC
LIMIT
	$3
OFFSET
	$2
`

type ListByTeamSlugParams struct {
	TeamSlug slug.Slug
	Offset   int32
	Limit    int32
}

type ListByTeamSlugRow struct {
	Deployment Deployment
	TotalCount int64
}

func (q *Queries) ListByTeamSlug(ctx context.Context, arg ListByTeamSlugParams) ([]*ListByTeamSlugRow, error) {
	rows, err := q.db.Query(ctx, listByTeamSlug, arg.TeamSlug, arg.Offset, arg.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []*ListByTeamSlugRow{}
	for rows.Next() {
		var i ListByTeamSlugRow
		if err := rows.Scan(
			&i.Deployment.ID,
			&i.Deployment.ExternalID,
			&i.Deployment.CreatedAt,
			&i.Deployment.TeamSlug,
			&i.Deployment.Repository,
			&i.Deployment.CommitSha,
			&i.Deployment.DeployerUsername,
			&i.Deployment.TriggerUrl,
			&i.Deployment.EnvironmentName,
			&i.TotalCount,
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

const listDeploymentResourcesByIDs = `-- name: ListDeploymentResourcesByIDs :many
SELECT
	id, created_at, deployment_id, "group", version, kind, name, namespace
FROM
	deployment_k8s_resources
WHERE
	id = ANY ($1::UUID[])
ORDER BY
	id
`

func (q *Queries) ListDeploymentResourcesByIDs(ctx context.Context, ids []uuid.UUID) ([]*DeploymentK8sResource, error) {
	rows, err := q.db.Query(ctx, listDeploymentResourcesByIDs, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []*DeploymentK8sResource{}
	for rows.Next() {
		var i DeploymentK8sResource
		if err := rows.Scan(
			&i.ID,
			&i.CreatedAt,
			&i.DeploymentID,
			&i.Group,
			&i.Version,
			&i.Kind,
			&i.Name,
			&i.Namespace,
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

const listDeploymentStatusesByIDs = `-- name: ListDeploymentStatusesByIDs :many
SELECT
	id, created_at, deployment_id, state, message
FROM
	deployment_statuses
WHERE
	id = ANY ($1::UUID[])
ORDER BY
	id
`

func (q *Queries) ListDeploymentStatusesByIDs(ctx context.Context, ids []uuid.UUID) ([]*DeploymentStatus, error) {
	rows, err := q.db.Query(ctx, listDeploymentStatusesByIDs, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []*DeploymentStatus{}
	for rows.Next() {
		var i DeploymentStatus
		if err := rows.Scan(
			&i.ID,
			&i.CreatedAt,
			&i.DeploymentID,
			&i.State,
			&i.Message,
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

const listForWorkload = `-- name: ListForWorkload :many
SELECT
	deployments.id, deployments.external_id, deployments.created_at, deployments.team_slug, deployments.repository, deployments.commit_sha, deployments.deployer_username, deployments.trigger_url, deployments.environment_name,
	COUNT(*) OVER () AS total_count
FROM
	deployments
	JOIN deployment_k8s_resources ON deployments.id = deployment_k8s_resources.deployment_id
WHERE
	deployment_k8s_resources.name = $1
	AND deployment_k8s_resources.kind = $2
	AND deployments.environment_name = $3
	AND deployments.team_slug = $4
ORDER BY
	deployments.created_at DESC
LIMIT
	$6
OFFSET
	$5
`

type ListForWorkloadParams struct {
	WorkloadName    string
	WorkloadKind    string
	EnvironmentName string
	TeamSlug        slug.Slug
	Offset          int32
	Limit           int32
}

type ListForWorkloadRow struct {
	Deployment Deployment
	TotalCount int64
}

func (q *Queries) ListForWorkload(ctx context.Context, arg ListForWorkloadParams) ([]*ListForWorkloadRow, error) {
	rows, err := q.db.Query(ctx, listForWorkload,
		arg.WorkloadName,
		arg.WorkloadKind,
		arg.EnvironmentName,
		arg.TeamSlug,
		arg.Offset,
		arg.Limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []*ListForWorkloadRow{}
	for rows.Next() {
		var i ListForWorkloadRow
		if err := rows.Scan(
			&i.Deployment.ID,
			&i.Deployment.ExternalID,
			&i.Deployment.CreatedAt,
			&i.Deployment.TeamSlug,
			&i.Deployment.Repository,
			&i.Deployment.CommitSha,
			&i.Deployment.DeployerUsername,
			&i.Deployment.TriggerUrl,
			&i.Deployment.EnvironmentName,
			&i.TotalCount,
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

const listResourcesForDeployment = `-- name: ListResourcesForDeployment :many
SELECT
	deployment_k8s_resources.id, deployment_k8s_resources.created_at, deployment_k8s_resources.deployment_id, deployment_k8s_resources."group", deployment_k8s_resources.version, deployment_k8s_resources.kind, deployment_k8s_resources.name, deployment_k8s_resources.namespace,
	COUNT(*) OVER () AS total_count
FROM
	deployment_k8s_resources
WHERE
	deployment_id = $1
ORDER BY
	created_at DESC
LIMIT
	$3
OFFSET
	$2
`

type ListResourcesForDeploymentParams struct {
	DeploymentID uuid.UUID
	Offset       int32
	Limit        int32
}

type ListResourcesForDeploymentRow struct {
	DeploymentK8sResource DeploymentK8sResource
	TotalCount            int64
}

func (q *Queries) ListResourcesForDeployment(ctx context.Context, arg ListResourcesForDeploymentParams) ([]*ListResourcesForDeploymentRow, error) {
	rows, err := q.db.Query(ctx, listResourcesForDeployment, arg.DeploymentID, arg.Offset, arg.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []*ListResourcesForDeploymentRow{}
	for rows.Next() {
		var i ListResourcesForDeploymentRow
		if err := rows.Scan(
			&i.DeploymentK8sResource.ID,
			&i.DeploymentK8sResource.CreatedAt,
			&i.DeploymentK8sResource.DeploymentID,
			&i.DeploymentK8sResource.Group,
			&i.DeploymentK8sResource.Version,
			&i.DeploymentK8sResource.Kind,
			&i.DeploymentK8sResource.Name,
			&i.DeploymentK8sResource.Namespace,
			&i.TotalCount,
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

const listStatusesForDeployment = `-- name: ListStatusesForDeployment :many
SELECT
	deployment_statuses.id, deployment_statuses.created_at, deployment_statuses.deployment_id, deployment_statuses.state, deployment_statuses.message,
	COUNT(*) OVER () AS total_count
FROM
	deployment_statuses
WHERE
	deployment_id = $1
ORDER BY
	created_at DESC
LIMIT
	$3
OFFSET
	$2
`

type ListStatusesForDeploymentParams struct {
	DeploymentID uuid.UUID
	Offset       int32
	Limit        int32
}

type ListStatusesForDeploymentRow struct {
	DeploymentStatus DeploymentStatus
	TotalCount       int64
}

func (q *Queries) ListStatusesForDeployment(ctx context.Context, arg ListStatusesForDeploymentParams) ([]*ListStatusesForDeploymentRow, error) {
	rows, err := q.db.Query(ctx, listStatusesForDeployment, arg.DeploymentID, arg.Offset, arg.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []*ListStatusesForDeploymentRow{}
	for rows.Next() {
		var i ListStatusesForDeploymentRow
		if err := rows.Scan(
			&i.DeploymentStatus.ID,
			&i.DeploymentStatus.CreatedAt,
			&i.DeploymentStatus.DeploymentID,
			&i.DeploymentStatus.State,
			&i.DeploymentStatus.Message,
			&i.TotalCount,
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
