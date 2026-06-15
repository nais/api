package graph

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/environment"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
	corev1 "k8s.io/api/core/v1"
)

func (r *containerImageResolver) Digest(ctx context.Context, obj *workload.ContainerImage) (*string, error) {
	var pods []*corev1.Pod
	var err error
	switch obj.Workload.Type {
	case workload.TypeApplication:
		pods, err = workload.ListAllPods(ctx, obj.Workload.EnvironmentName, obj.Workload.TeamSlug, obj.Workload.Name)
	case workload.TypeJob:
		pods, err = workload.ListAllPodsForJob(ctx, obj.Workload.EnvironmentName, obj.Workload.TeamSlug, obj.Workload.Name)
	}

	if err != nil {
		return nil, err
	}

	activePods := make([]*corev1.Pod, 0)
	for _, pod := range pods {
		if pod.DeletionTimestamp != nil {
			continue
		}
		for _, c := range pod.Spec.Containers {
			if c.Image == obj.Ref() {
				activePods = append(activePods, pod)
				break
			}
		}
	}

	// Sort newest first: during rollouts, the newest pod represents the target image.
	slices.SortFunc(activePods, func(a, b *corev1.Pod) int {
		return b.CreationTimestamp.Compare(a.CreationTimestamp.Time)
	})

	for _, pod := range activePods {
		for _, status := range pod.Status.ContainerStatuses {
			if status.Image == obj.Ref() {
				_, digest, ok := strings.Cut(status.ImageID, "@")
				if ok {
					return &digest, nil
				}
			}
		}
	}

	return nil, nil
}

func (r *containerImageResolver) ActivityLog(ctx context.Context, obj *workload.ContainerImage, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, filter *activitylog.ActivityLogFilter) (*activitylog.ActivityLogEntryConnection, error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}
	return activitylog.ListForResource(ctx, "VULNERABILITY", obj.Name, page, filter)
}

func (r *environmentResolver) Workloads(ctx context.Context, obj *environment.Environment, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *workload.EnvironmentWorkloadOrder) (*pagination.Connection[workload.Workload], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	apps := application.ListAllInEnvironment(ctx, obj.Name)
	jobs := job.ListAllInEnvironment(ctx, obj.Name)

	workloads := make([]workload.Workload, 0, len(apps)+len(jobs))
	for _, app := range apps {
		workloads = append(workloads, app)
	}
	for _, j := range jobs {
		workloads = append(workloads, j)
	}

	if orderBy == nil {
		orderBy = &workload.EnvironmentWorkloadOrder{
			Field:     "NAME",
			Direction: model.OrderDirectionAsc,
		}
	}
	workload.SortFilterEnvironment.Sort(ctx, workloads, orderBy.Field, orderBy.Direction)

	ret := pagination.Slice(workloads, page)
	return pagination.NewConnection(ret, page, len(workloads)), nil
}

func (r *teamResolver) Workloads(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *workload.WorkloadOrder, filter *workload.TeamWorkloadsFilter) (*pagination.Connection[workload.Workload], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	apps := application.ListAllForTeam(ctx, obj.Slug, nil, nil)
	jobs := job.ListAllForTeam(ctx, obj.Slug, nil, nil)

	workloads := make([]workload.Workload, 0, len(apps)+len(jobs))
	for _, app := range apps {
		workloads = append(workloads, app)
	}
	for _, j := range jobs {
		workloads = append(workloads, j)
	}

	filtered := workload.SortFilter.Filter(ctx, workloads, filter)
	if orderBy == nil {
		orderBy = &workload.WorkloadOrder{
			Field:     "NAME",
			Direction: model.OrderDirectionAsc,
		}
	}
	workload.SortFilter.Sort(ctx, filtered, orderBy.Field, orderBy.Direction)

	ret := pagination.Slice(filtered, page)
	return pagination.NewConnection(ret, page, len(filtered)), nil
}

func (r *teamEnvironmentResolver) Workload(ctx context.Context, obj *team.TeamEnvironment, name string) (workload.Workload, error) {
	return tryWorkload(ctx, obj.TeamSlug, obj.EnvironmentName, name)
}

func (r *Resolver) ContainerImage() gengql.ContainerImageResolver { return &containerImageResolver{r} }

type containerImageResolver struct{ *Resolver }
