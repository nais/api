package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/nais/api/internal/kubernetes/watcher"
	nais_io_v1 "github.com/nais/pgrator/pkg/api/v1"
)

// PhysicalStatus selects exactly one source. Once clusterName is published,
// direct status remains authoritative even if it is incomplete or degraded.
func PhysicalStatus(ctx context.Context, pg *Postgres) (PostgresState, *PostgresConfiguration, error) {
	if pg.directClusterName != nil {
		return pg.physicalState, pg.configuration, nil
	}
	branch, err := legacyPhysicalBranch(ctx, pg)
	if errors.Is(err, &watcher.ErrorNotFound{}) {
		return PostgresBranchStateProgressing, nil, nil
	}
	if err != nil {
		return "", nil, err
	}
	return branch.State, branch.Configuration, nil
}

// physicalClusterName selects the same internal target before and after direct ownership.
func physicalClusterName(ctx context.Context, pg *Postgres) (*string, error) {
	if pg.directClusterName != nil {
		return pg.directClusterName, nil
	}
	branch, err := legacyPhysicalBranch(ctx, pg)
	if errors.Is(err, &watcher.ErrorNotFound{}) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return new(branch.clusterName()), nil
}

func legacyPhysicalBranch(ctx context.Context, pg *Postgres) (*PostgresBranch, error) {
	return legacyBranch(ctx, pg.TeamSlug.String(), pg.EnvironmentName, pg.Name, observedBranchName(pg))
}

func observedBranchName(pg *Postgres) string {
	if pg.directClusterName == nil && pg.ActiveBranch != nil && *pg.ActiveBranch != "" {
		return *pg.ActiveBranch
	}
	return nais_io_v1.DefaultBranchName
}

// PostgresEarliestRestoreTime uses the same physical target as database status,
// including the preserved archive identity of an adopted main.
func PostgresEarliestRestoreTime(ctx context.Context, pg *Postgres) (*time.Time, error) {
	if pg.directClusterName != nil {
		return EarliestRestoreTime(ctx, mainBranchView(pg))
	}
	branch, err := legacyPhysicalBranch(ctx, pg)
	if errors.Is(err, &watcher.ErrorNotFound{}) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return EarliestRestoreTime(ctx, branch)
}

// PhysicalIssueName retains the existing issue key while clients move from a
// branch relation to Postgres. Stored issue identities need not be rewritten
// together with the Kubernetes ownership change.
func PhysicalIssueName(pg *Postgres) string {
	if pg.directClusterName != nil || pg.ActiveBranch == nil {
		return nais_io_v1.PostgresPhysicalName(pg.Name)
	}
	return nais_io_v1.PostgresBranchObjectName(pg.Name, *pg.ActiveBranch)
}

func mainBranchView(pg *Postgres) *PostgresBranch {
	return &PostgresBranch{
		Name: nais_io_v1.DefaultBranchName, PostgresName: pg.Name,
		TeamSlug: pg.TeamSlug, EnvironmentName: pg.EnvironmentName,
		CreatedAt: pg.CreatedAt, Labels: pg.Labels,
		State: pg.physicalState, Configuration: pg.configuration,
		ClusterName: *pg.directClusterName,
	}
}

func (branch *PostgresBranch) clusterName() string {
	if branch.ClusterName != "" {
		return branch.ClusterName
	}
	return nais_io_v1.CNPGClusterName(branch.GetName())
}

// BranchViews retains legacy branches until a Postgres explicitly selects the
// direct model. Direct databases expose only a virtual main with the same ID.
func BranchViews(postgreses []*Postgres, branches []*PostgresBranch) []*PostgresBranch {
	type key struct{ environment, namespace, name string }
	direct := make(map[key]*Postgres)
	for _, pg := range postgreses {
		if pg.directClusterName != nil {
			direct[key{pg.EnvironmentName, pg.TeamSlug.String(), pg.Name}] = pg
		}
	}
	views := make([]*PostgresBranch, 0, len(branches)+len(direct))
	for _, branch := range branches {
		if direct[key{branch.EnvironmentName, branch.TeamSlug.String(), branch.PostgresName}] == nil {
			views = append(views, branch)
		}
	}
	for _, pg := range direct {
		views = append(views, mainBranchView(pg))
	}
	return views
}
