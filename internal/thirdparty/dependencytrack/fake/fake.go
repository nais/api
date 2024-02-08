package fake

import (
	"context"
	"math/rand"

	"github.com/google/uuid"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/thirdparty/dependencytrack"
)

type FakeDependencytrackClient struct{}

func New() *FakeDependencytrackClient {
	return &FakeDependencytrackClient{}
}

var mapOfApps = map[string]uuid.UUID{}

func (f *FakeDependencytrackClient) VulnerabilitySummary(ctx context.Context, app *dependencytrack.AppInstance) (*model.Vulnerability, error) {
	critical := rand.Intn(10)
	high := rand.Intn(10)
	medium := rand.Intn(10)
	low := rand.Intn(10)
	unassigned := rand.Intn(10)

	return &model.Vulnerability{
		ID:           scalar.VulnerabilitiesIdent(app.ID()),
		AppName:      app.App,
		Env:          app.Env,
		FindingsLink: "https://dependencytrack.example.com",
		Summary: &model.VulnerabilitySummary{
			Total:      critical + high + medium + low + unassigned,
			RiskScore:  ((critical * 10) + (high * 5) + (medium * 3) + (low * 1) + (unassigned * 5)),
			Critical:   critical,
			High:       high,
			Medium:     medium,
			Low:        low,
			Unassigned: unassigned,
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

func (f *FakeDependencytrackClient) GetProjectMetrics(ctx context.Context, app *dependencytrack.AppInstance) (*model.VulnerabilityMetricsWithProjectID, error) {
	critical := rand.Intn(10)
	high := rand.Intn(10)
	medium := rand.Intn(10)
	low := rand.Intn(10)
	unassigned := rand.Intn(10)

	var uuId uuid.UUID
	if mapOfApps[app.ID()] == uuid.Nil {
		uuId = uuid.New()
		mapOfApps[app.ID()] = uuId
	} else {
		uuId = mapOfApps[app.ID()]
	}

	return &model.VulnerabilityMetricsWithProjectID{
		ProjectID: scalar.VulnerabilitiesIdent(uuId.String()),
		VulnerabilitySummary: &model.VulnerabilitySummary{
			Total:      critical + high + medium + low + unassigned,
			RiskScore:  ((critical * 10) + (high * 5) + (medium * 3) + (low * 1) + (unassigned * 5)),
			Critical:   critical,
			High:       high,
			Medium:     medium,
			Low:        low,
			Unassigned: unassigned,
		},
	}, nil
}
