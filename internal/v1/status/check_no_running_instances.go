package status

import (
	"context"

	"github.com/nais/api/internal/v1/workload"
	"github.com/nais/api/internal/v1/workload/application"
)

type checkAppNoRunningInstances struct{}

func (c checkAppNoRunningInstances) Run(ctx context.Context, w workload.Workload) ([]WorkloadStatusError, WorkloadState) {
	s := c.run(ctx, w)
	if s == nil {
		return nil, WorkloadStateNais
	}
	return []WorkloadStatusError{s}, WorkloadStateFailing
}

func (checkAppNoRunningInstances) run(ctx context.Context, w workload.Workload) WorkloadStatusError {
	app := w.(*application.Application)

	instances, err := application.ListAllInstances(ctx, app.EnvironmentName, app.TeamSlug, app.Name)
	if err != nil {
		return nil
	}

	failingInstances := failingInstances(instances)

	resources := app.Resources()
	if (len(instances) == 0 || len(failingInstances) == len(instances)) && resources.Scaling.MinInstances > 0 && resources.Scaling.MaxInstances > 0 {
		return &WorkloadStatusNoRunningInstances{
			Level: WorkloadStatusErrorLevelError,
		}
	}

	return nil
}

func (checkAppNoRunningInstances) Supports(w workload.Workload) bool {
	_, ok := w.(*application.Application)
	return ok
}

func failingInstances(instances []*application.Instance) []*application.Instance {
	ret := []*application.Instance{}
	for _, instance := range instances {
		if instance.State() == application.InstanceStateFailing {
			ret = append(ret, instance)
		}
	}
	return ret
}
