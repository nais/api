package job

import (
	"context"
	"slices"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/modelv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/kubernetes/watcher"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination, orderBy *JobOrder) (*JobConnection, error) {
	ret := ListAllForTeam(ctx, teamSlug)
	if orderBy != nil {
		switch orderBy.Field {
		case JobOrderFieldName:
			slices.SortStableFunc(ret, func(a, b *Job) int {
				return modelv1.Compare(a.Name, b.Name, orderBy.Direction)
			})
		case JobOrderFieldEnvironment:
			slices.SortStableFunc(ret, func(a, b *Job) int {
				return modelv1.Compare(a.EnvironmentName, b.EnvironmentName, orderBy.Direction)
			})
		case JobOrderFieldDeploymentTime:
			panic("not implemented yet")
		case JobOrderFieldStatus:
			panic("not implemented yet")
		}
	}

	jobs := pagination.Slice(ret, page)
	return pagination.NewConnection(jobs, page, int32(len(ret))), nil
}

func ListAllForTeam(ctx context.Context, teamSlug slug.Slug) []*Job {
	allJobs := fromContext(ctx).jobWatcher.GetByNamespace(teamSlug.String())
	ret := make([]*Job, len(allJobs))
	for i, obj := range allJobs {
		ret[i] = toGraphJob(obj.Obj, obj.Cluster)
	}

	return ret
}

func Get(ctx context.Context, teamSlug slug.Slug, environment, name string) (*Job, error) {
	job, err := fromContext(ctx).jobWatcher.Get(environment, teamSlug.String(), name)
	if err != nil {
		return nil, err
	}
	return toGraphJob(job, environment), nil
}

func GetJobRun(ctx context.Context, teamSlug slug.Slug, environment, name string) (*JobRun, error) {
	run, err := fromContext(ctx).runWatcher.Get(environment, teamSlug.String(), name)
	if err != nil {
		return nil, err
	}
	return toGraphJobRun(run, environment), nil
}

func GetByIdent(ctx context.Context, id ident.Ident) (*Job, error) {
	teamSlug, env, name, err := parseIdent(id)
	if err != nil {
		return nil, err
	}
	return Get(ctx, teamSlug, env, name)
}

func GetByJobRunIdent(ctx context.Context, id ident.Ident) (*JobRun, error) {
	teamSlug, env, name, err := parseIdent(id)
	if err != nil {
		return nil, err
	}
	return GetJobRun(ctx, teamSlug, env, name)
}

func Runs(ctx context.Context, teamSlug slug.Slug, jobName string, page *pagination.Pagination) (*JobRunConnection, error) {
	nameReq, err := labels.NewRequirement("app", selection.Equals, []string{jobName})
	if err != nil {
		return nil, err
	}

	selector := labels.NewSelector().Add(*nameReq)

	allRuns := fromContext(ctx).runWatcher.GetByNamespace(teamSlug.String(), watcher.WithLabels(selector))
	ret := make([]*JobRun, len(allRuns))
	for i, run := range allRuns {
		ret[i] = toGraphJobRun(run.Obj, run.Cluster)
	}

	slices.SortStableFunc(ret, func(a, b *JobRun) int {
		return b.CreationTime.Compare(a.CreationTime)
	})

	runs := pagination.Slice(ret, page)
	return pagination.NewConnection(runs, page, int32(len(ret))), nil
}
