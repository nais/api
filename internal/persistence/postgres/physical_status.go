package postgres

import (
	"context"

	nais_io_v1 "github.com/nais/pgrator/pkg/api/v1"
)

// PhysicalStatus reads only Postgres status. Missing physical status means the
// database is still provisioning; a retired branch must never supply readiness.
func PhysicalStatus(ctx context.Context, pg *Postgres) (PostgresState, *PostgresConfiguration, error) {
	if pg.directClusterName != nil {
		return pg.physicalState, pg.configuration, nil
	}
	return PostgresBranchStateProgressing, nil, nil
}

// physicalClusterName selects the same internal target before and after direct ownership.
func physicalClusterName(ctx context.Context, pg *Postgres) (*string, error) {
	return pg.directClusterName, nil
}

func observedBranchName(pg *Postgres) string {
	return nais_io_v1.DefaultBranchName
}

func mainBranchView(pg *Postgres) *PostgresBranch {
	clusterName := ""
	state := PostgresBranchStateProgressing
	if pg.directClusterName != nil {
		clusterName = *pg.directClusterName
		state = pg.physicalState
	}
	return &PostgresBranch{
		Name: nais_io_v1.DefaultBranchName, PostgresName: pg.Name,
		TeamSlug: pg.TeamSlug, EnvironmentName: pg.EnvironmentName,
		CreatedAt: pg.CreatedAt, Labels: pg.Labels,
		State: state, Configuration: pg.configuration,
		ClusterName: clusterName,
	}
}

func (branch *PostgresBranch) clusterName() string {
	if branch.ClusterName != "" {
		return branch.ClusterName
	}
	return nais_io_v1.CNPGClusterName(branch.GetName())
}

// MainViews retains the deprecated main view, including its stable node ID,
// using logical Postgres resources only.
func MainViews(postgreses []*Postgres) []*PostgresBranch {
	views := make([]*PostgresBranch, 0, len(postgreses))
	for _, pg := range postgreses {
		views = append(views, mainBranchView(pg))
	}
	return views
}
