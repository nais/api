package fake

import (
	"context"
	"math/rand"
	"time"

	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/k8s"
	"github.com/nais/api/internal/slug"
)

type FakeResourceUsageClient struct {
	db        database.TeamRepo
	k8sClient *k8s.Client
}

func New(db database.TeamRepo, k8sClient *k8s.Client) *FakeResourceUsageClient {
	return &FakeResourceUsageClient{
		db:        db,
		k8sClient: k8sClient,
	}
}

func (f *FakeResourceUsageClient) TeamsUtilization(ctx context.Context, resourceType model.UsageResourceType) ([]*model.TeamUtilizationData, error) {
	teamSlugs, err := f.db.GetAllTeamSlugs(ctx)
	if err != nil {
		return nil, err
	}

	ret := make([]*model.TeamUtilizationData, len(teamSlugs))
	for i, teamSlug := range teamSlugs {
		ret[i] = &model.TeamUtilizationData{
			TeamSlug: teamSlug,
		}
		if resourceType == model.UsageResourceTypeCPU {
			ret[i].Requested = between(6, 90)
			ret[i].Used = between(1, 6)
		} else {
			ret[i].Requested = between(50000000000, 200000000000)
			ret[i].Used = between(2000000000, 50000000000)
		}
	}
	return ret, nil
}

func (f *FakeResourceUsageClient) TeamUtilization(ctx context.Context, teamSlug slug.Slug, resourceType model.UsageResourceType) ([]*model.AppUtilizationData, error) {
	apps, err := f.k8sClient.Apps(ctx, teamSlug.String())
	if err != nil {
		return nil, err
	}

	ret := make([]*model.AppUtilizationData, len(apps))
	for i, app := range apps {
		ret[i] = &model.AppUtilizationData{
			TeamSlug: teamSlug,
			AppName:  app.Name,
			Env:      app.Env.Name,
		}
		if resourceType == model.UsageResourceTypeCPU {
			ret[i].Requested = between(0.2, 5)
			ret[i].Used = between(0.01, 0.2)
		} else {
			ret[i].Requested = between(500000000, 4000000000)
			ret[i].Used = between(500000000, 2000000000)
		}
	}

	return ret, nil
}

func (f *FakeResourceUsageClient) AppResourceRequest(ctx context.Context, env string, teamSlug slug.Slug, app string, resourceType model.UsageResourceType) (float64, error) {
	if resourceType == model.UsageResourceTypeCPU {
		return between(0.4, 10), nil
	}
	return between(20000000, 50000000), nil
}

func (f *FakeResourceUsageClient) AppResourceUsage(ctx context.Context, env string, teamSlug slug.Slug, app string, resourceType model.UsageResourceType) (float64, error) {
	if resourceType == model.UsageResourceTypeCPU {
		return between(0.2, 5), nil
	}
	return between(5000000, 20000000), nil
}

func (f *FakeResourceUsageClient) AppResourceUsageRange(ctx context.Context, env string, teamSlug slug.Slug, app string, resourceType model.UsageResourceType, start time.Time, end time.Time, step int) ([]*model.UsageDataPoint, error) {
	ret := []*model.UsageDataPoint{}
	for i := 0; i < 200; i++ {
		ret = append(ret, &model.UsageDataPoint{
			Timestamp: start.Add(time.Duration(i) * time.Minute),
		})
		if resourceType == model.UsageResourceTypeCPU {
			ret[i].Value = between(0.2, 5)
		} else {
			ret[i].Value = between(5000000, 20000000)
		}
	}
	return ret, nil
}

func between(min, max float64) float64 {
	return min + rand.Float64()*(max-min)
}
