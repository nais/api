package graphv1

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/v1/cost"
	"github.com/nais/api/internal/v1/team"
	"github.com/nais/api/internal/v1/workload/application"
	"github.com/nais/api/internal/v1/workload/job"
)

func (r *applicationResolver) Cost(ctx context.Context, obj *application.Application) (*cost.WorkloadCost, error) {
	panic(fmt.Errorf("not implemented: Cost - cost"))
}

func (r *jobResolver) Cost(ctx context.Context, obj *job.Job) (*cost.WorkloadCost, error) {
	panic(fmt.Errorf("not implemented: Cost - cost"))
}

func (r *teamResolver) Cost(ctx context.Context, obj *team.Team) (*cost.TeamCost, error) {
	panic(fmt.Errorf("not implemented: Cost - cost"))
}
