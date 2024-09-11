package vulnerabilities

import (
	"context"
	"fmt"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
)

func NewFake(client Client) Client {
	return fakeDependencyTrackClient{client}
}

type fakeDependencyTrackClient struct {
	client Client
}

func (f fakeDependencyTrackClient) GetMetadataForImageByProjectID(ctx context.Context, projectID string) (*model.ImageDetails, error) {
	return f.client.GetMetadataForImageByProjectID(ctx, projectID)
}

func (f fakeDependencyTrackClient) GetMetadataForImage(ctx context.Context, image string) (*model.ImageDetails, error) {
	return f.client.GetMetadataForImage(ctx, image)
}

func (f fakeDependencyTrackClient) GetFindingsForImageByProjectID(ctx context.Context, projectID string, suppressed bool) ([]*model.Finding, error) {
	return f.client.GetFindingsForImageByProjectID(ctx, projectID, suppressed)
}

func (f fakeDependencyTrackClient) GetMetadataForTeam(ctx context.Context, team string) ([]*model.ImageDetails, error) {
	images, err := f.client.GetMetadataForTeam(ctx, team)
	if err != nil {
		return nil, err
	}

	retVal := make([]*model.ImageDetails, 0)
	for _, image := range images {
		for i := range 20 {
			env := "dev"
			workloadType := "app"
			if i%2 == 0 {
				env = "superprod"
				workloadType = "job"
			}

			name := fmt.Sprintf("random-%s-%d", workloadType, i)
			team := "devteam"
			id := fmt.Sprintf("workload:%s|%s|%s|%s-%d", env, team, workloadType, name, i)
			image.GQLVars.WorkloadReferences = append(image.GQLVars.WorkloadReferences, &model.WorkloadReference{
				ID:           scalar.WorkloadIdent(id),
				Name:         name,
				Team:         team,
				WorkloadType: workloadType,
				Environment:  env,
			})
		}
		retVal = append(retVal, image)
	}
	return retVal, nil
}

func (f fakeDependencyTrackClient) GetVulnerabilityStatus(ctx context.Context, image string) (model.StateError, error) {
	return f.client.GetVulnerabilityStatus(ctx, image)
}

func (f fakeDependencyTrackClient) SuppressFinding(ctx context.Context, analysisState, comment, componentID, projectID, vulnerabilityID, suppressedBy string, suppress bool) (*model.AnalysisTrail, error) {
	return f.client.SuppressFinding(ctx, analysisState, comment, componentID, projectID, vulnerabilityID, suppressedBy, suppress)
}

func (f fakeDependencyTrackClient) GetAnalysisTrailForImage(ctx context.Context, projectID, componentID, vulnerabilityID string) (*model.AnalysisTrail, error) {
	return f.client.GetAnalysisTrailForImage(ctx, projectID, componentID, vulnerabilityID)
}

func (f fakeDependencyTrackClient) UploadProject(ctx context.Context, image, name, version, team string, bom []byte) error {
	return f.client.UploadProject(ctx, image, name, version, team, bom)
}

var _ Client = (*fakeDependencyTrackClient)(nil)
