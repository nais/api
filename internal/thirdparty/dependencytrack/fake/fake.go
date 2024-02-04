package fake

import (
	"context"
	"math/rand"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/thirdparty/dependencytrack"
)

type FakeDependencytrackClient struct{}

func New() *FakeDependencytrackClient {
	return &FakeDependencytrackClient{}
}

func (f *FakeDependencytrackClient) VulnerabilitySummary(ctx context.Context, app *dependencytrack.AppInstance) (*model.Vulnerability, error) {
	return &model.Vulnerability{
		ID:           scalar.VulnerabilitiesIdent(app.ID()),
		AppName:      app.App,
		Env:          app.Env,
		FindingsLink: "https://dependencytrack.example.com",
		Summary: &model.VulnerabilitySummary{
			Total:      rand.Intn(100),
			RiskScore:  rand.Intn(20),
			Critical:   rand.Intn(10),
			High:       rand.Intn(10),
			Medium:     rand.Intn(10),
			Low:        rand.Intn(10),
			Unassigned: rand.Intn(10),
		},
		HasBom: rand.Intn(4) != 0,
	}, nil
}

func (f *FakeDependencytrackClient) GetVulnerabilities(ctx context.Context, apps []*dependencytrack.AppInstance) ([]*model.Vulnerability, error) {
	ret := make([]*model.Vulnerability, len(apps))
	for i, app := range apps {
		ret[i], _ = f.VulnerabilitySummary(ctx, app)
	}
	return ret, nil
}

func (f *FakeDependencytrackClient) GetProjectMetrics(ctx context.Context, app *dependencytrack.AppInstance) (*model.VulnerabilityMetrics, error) {
	return &model.VulnerabilityMetrics{
		ProjectId: app.ID(),
		VulnerabilitySummary: &model.VulnerabilitySummary{
			Total:      rand.Intn(100),
			RiskScore:  rand.Intn(20),
			Critical:   rand.Intn(10),
			High:       rand.Intn(10),
			Medium:     rand.Intn(10),
			Low:        rand.Intn(10),
			Unassigned: rand.Intn(10),
		},
	}, nil
}
