// Code generated by sqlc. DO NOT EDIT.
// source: deployments.sql

package grpcdeploymentsql

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/api/internal/slug"
)

const createDeployment = `-- name: CreateDeployment :one
INSERT INTO
	deployments (
		external_id,
		created_at,
		team_slug,
		repository,
		environment_name,
		commit_sha,
		deployer_username,
		trigger_url
	)
VALUES
	(
		$1,
		COALESCE($2, CLOCK_TIMESTAMP())::TIMESTAMPTZ,
		$3,
		$4,
		$5,
		$6,
		$7,
		$8
	)
RETURNING
	id
`

type CreateDeploymentParams struct {
	ExternalID       *string
	CreatedAt        pgtype.Timestamptz
	TeamSlug         slug.Slug
	Repository       *string
	EnvironmentName  string
	CommitSha        *string
	DeployerUsername *string
	TriggerUrl       *string
}

func (q *Queries) CreateDeployment(ctx context.Context, arg CreateDeploymentParams) (uuid.UUID, error) {
	row := q.db.QueryRow(ctx, createDeployment,
		arg.ExternalID,
		arg.CreatedAt,
		arg.TeamSlug,
		arg.Repository,
		arg.EnvironmentName,
		arg.CommitSha,
		arg.DeployerUsername,
		arg.TriggerUrl,
	)
	var id uuid.UUID
	err := row.Scan(&id)
	return id, err
}

const createDeploymentK8sResource = `-- name: CreateDeploymentK8sResource :one
INSERT INTO
	deployment_k8s_resources (
		deployment_id,
		"group",
		version,
		kind,
		name,
		namespace
	)
VALUES
	(
		(
			SELECT
				deployments.id
			FROM
				deployments
			WHERE
				deployments.id = $1
				OR deployments.external_id = $2
		),
		$3,
		$4,
		$5,
		$6,
		$7
	)
RETURNING
	id
`

type CreateDeploymentK8sResourceParams struct {
	DeploymentID         uuid.UUID
	ExternalDeploymentID *string
	Group                string
	Version              string
	Kind                 string
	Name                 string
	Namespace            string
}

func (q *Queries) CreateDeploymentK8sResource(ctx context.Context, arg CreateDeploymentK8sResourceParams) (uuid.UUID, error) {
	row := q.db.QueryRow(ctx, createDeploymentK8sResource,
		arg.DeploymentID,
		arg.ExternalDeploymentID,
		arg.Group,
		arg.Version,
		arg.Kind,
		arg.Name,
		arg.Namespace,
	)
	var id uuid.UUID
	err := row.Scan(&id)
	return id, err
}

const createDeploymentStatus = `-- name: CreateDeploymentStatus :one
INSERT INTO
	deployment_statuses (created_at, deployment_id, state, message)
VALUES
	(
		COALESCE($1, CLOCK_TIMESTAMP())::TIMESTAMPTZ,
		(
			SELECT
				deployments.id
			FROM
				deployments
			WHERE
				deployments.id = $2
				OR deployments.external_id = $3
		),
		$4,
		$5
	)
RETURNING
	id
`

type CreateDeploymentStatusParams struct {
	CreatedAt            pgtype.Timestamptz
	DeploymentID         uuid.UUID
	ExternalDeploymentID *string
	State                DeploymentState
	Message              string
}

func (q *Queries) CreateDeploymentStatus(ctx context.Context, arg CreateDeploymentStatusParams) (uuid.UUID, error) {
	row := q.db.QueryRow(ctx, createDeploymentStatus,
		arg.CreatedAt,
		arg.DeploymentID,
		arg.ExternalDeploymentID,
		arg.State,
		arg.Message,
	)
	var id uuid.UUID
	err := row.Scan(&id)
	return id, err
}

const teamExists = `-- name: TeamExists :one
SELECT
	EXISTS (
		SELECT
			1
		FROM
			teams
		WHERE
			slug = $1
	)
`

func (q *Queries) TeamExists(ctx context.Context, argSlug slug.Slug) (bool, error) {
	row := q.db.QueryRow(ctx, teamExists, argSlug)
	var exists bool
	err := row.Scan(&exists)
	return exists, err
}
