package fake

import (
	"context"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/thirdparty/dependencytrack"
	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
)

type FakeDependencytrackClient struct {
	client *dependencytrack.Client
	cache  *cache.Cache
}

func New(log logrus.FieldLogger) *FakeDependencytrackClient {
	f := &FakeDependencytrackClient{}
	f.cache = cache.New(24*time.Hour, 24*time.Hour)
	f.client = dependencytrack.
		New("endpoint", "username", "password", "frontend", log.WithField("client", "fake_dependencytrack")).
		WithCache(f.cache)

	return f
}

var mapOfApps = map[string]uuid.UUID{}

func (f *FakeDependencytrackClient) VulnerabilitySummary(ctx context.Context, app *dependencytrack.AppInstance) (*model.Vulnerability, error) {
	f.setCacheEntryForApp(app)
	return f.client.VulnerabilitySummary(ctx, app)
}

func (f *FakeDependencytrackClient) GetVulnerabilities(ctx context.Context, apps []*dependencytrack.AppInstance, filters ...dependencytrack.Filter) ([]*model.Vulnerability, error) {
	for _, app := range apps {
		f.setCacheEntryForApp(app)
	}
	return f.client.GetVulnerabilities(ctx, apps, filters...)
}

// TODO: Should use the cache so that we can call the innter client GetProjectMetrics function
func (f *FakeDependencytrackClient) GetProjectMetrics(ctx context.Context, app *dependencytrack.AppInstance, date string) (*dependencytrack.ProjectMetric, error) {
	id, ok := mapOfApps[app.ID()]
	if !ok {
		id = uuid.New()
		mapOfApps[app.ID()] = id
	}

	vulnMetrics := make([]*dependencytrack.VulnerabilityMetrics, 0)
	critical := rand.Intn(10)
	high := rand.Intn(10)
	medium := rand.Intn(10)
	low := rand.Intn(10)
	unassigned := rand.Intn(10)
	vulnMetrics = append(vulnMetrics, &dependencytrack.VulnerabilityMetrics{
		Total:           critical + high + medium + low + unassigned,
		RiskScore:       ((critical * 10) + (high * 5) + (medium * 3) + (low * 1) + (unassigned * 5)),
		Critical:        critical,
		High:            high,
		Medium:          medium,
		Low:             low,
		Unassigned:      unassigned,
		FirstOccurrence: 1705413522933,
		LastOccurrence:  1707463343762,
	})
	return &dependencytrack.ProjectMetric{
		ProjectID:            id,
		VulnerabilityMetrics: vulnMetrics,
	}, nil
}

func (f *FakeDependencytrackClient) setCacheEntryForApp(app *dependencytrack.AppInstance) {
	v := &model.Vulnerability{
		ID:           scalar.VulnerabilitiesIdent(app.ID()),
		AppName:      app.App,
		Env:          app.Env,
		FindingsLink: "https://dependencytrack.example.com",
	}

	switch rand.Intn(4) {
	case 0:
		v.HasBom = false
	case 1:
		v.HasBom = false
		v.Summary = &model.VulnerabilitySummary{
			RiskScore:  -1,
			Total:      -1,
			Critical:   -1,
			High:       -1,
			Medium:     -1,
			Low:        -1,
			Unassigned: -1,
		}
	default:
		critical := rand.Intn(10)
		high := rand.Intn(10)
		medium := rand.Intn(10)
		low := rand.Intn(10)
		unassigned := rand.Intn(10)

		v.Summary = &model.VulnerabilitySummary{
			Total:      critical + high + medium + low + unassigned,
			RiskScore:  (critical * 10) + (high * 5) + (medium * 3) + (low * 1) + (unassigned * 5),
			Critical:   critical,
			High:       high,
			Medium:     medium,
			Low:        low,
			Unassigned: unassigned,
		}
		v.HasBom = true
	}
	f.cache.Set(app.ID(), v, 24*time.Hour)
}
