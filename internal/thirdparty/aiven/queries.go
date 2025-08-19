package aiven

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/environmentmapper"
)

func GetProject(ctx context.Context, environmentName string) (Project, error) {
	clusterName := environmentmapper.ClusterName(environmentName)
	project, ok := fromContext(ctx).projects[clusterName]
	if !ok {
		return Project{}, fmt.Errorf("aiven project not found for cluster: %s", clusterName)
	}
	return project, nil
}
