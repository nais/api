package postgres

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/nais/api/internal/kubernetes/watcher"
	nais_io_v1 "github.com/nais/pgrator/pkg/api/v1"
)

type PostgresConfiguration struct {
	Parameters []*PostgresParameter `json:"parameters"`
}

type PostgresParameter struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func Configuration(ctx context.Context, branch *PostgresBranch) (*PostgresConfiguration, error) {
	clusterName := nais_io_v1.CNPGClusterName(nais_io_v1.PostgresBranchObjectName(branch.PostgresName, branch.Name))
	cluster, err := fromContext(ctx).cnpgClusterWatcher.Get(branch.EnvironmentName, branch.TeamSlug.String(), clusterName)
	if errors.Is(err, &watcher.ErrorNotFound{}) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting CNPG Cluster %q configuration: %w", clusterName, err)
	}
	parameters := make([]*PostgresParameter, 0, len(cluster.Parameters))
	for _, name := range slices.Sorted(maps.Keys(cluster.Parameters)) {
		parameters = append(parameters, &PostgresParameter{Name: name, Value: cluster.Parameters[name]})
	}
	return &PostgresConfiguration{Parameters: parameters}, nil
}
