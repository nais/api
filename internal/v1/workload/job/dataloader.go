package job

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/k8s"
	"github.com/nais/api/internal/v1/graphv1/loaderv1"
	"github.com/vikstrous/dataloadgen"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, k8sClient *k8s.Client, defaultOpts []dataloadgen.Option) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(k8sClient, defaultOpts))
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	k8sClient *k8s.Client
	jobLoader *dataloadgen.Loader[jobIdentifier, *Job]
}

func newLoaders(k8sClient *k8s.Client, opts []dataloadgen.Option) *loaders {
	jobLoader := &dataloader{
		k8sClient: k8sClient,
	}

	return &loaders{
		k8sClient: k8sClient,
		jobLoader: dataloadgen.NewLoader(jobLoader.list, opts...),
	}
}

type dataloader struct {
	k8sClient *k8s.Client
}

func (l dataloader) getJobs(ctx context.Context, ids []jobIdentifier) ([]*model.NaisJob, error) {
	ret := make([]*model.NaisJob, 0)
	for _, id := range ids {
		job, err := l.k8sClient.NaisJob(ctx, id.name, id.namespace, id.environment)
		if err != nil {
			fmt.Println("error fetching job", err)
			continue
		}
		ret = append(ret, job)
	}
	return ret, nil
}

type jobIdentifier struct {
	namespace   string
	environment string
	name        string
}

func (l dataloader) list(ctx context.Context, ids []jobIdentifier) ([]*Job, []error) {
	makeKey := func(obj *Job) jobIdentifier {
		return jobIdentifier{
			namespace:   obj.TeamSlug.String(),
			environment: obj.EnvironmentName,
			name:        obj.Name,
		}
	}
	return loaderv1.LoadModels(ctx, ids, l.getJobs, toGraphJob, makeKey)
}
