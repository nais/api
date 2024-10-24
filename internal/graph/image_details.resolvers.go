package graph

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
)

func (r *analysisTrailResolver) Comments(ctx context.Context, obj *model.AnalysisTrail, offset *int, limit *int) (*model.AnalysisCommentList, error) {
	page := model.NewPagination(offset, limit)

	nodes, pageInfo := model.PaginatedSlice(obj.GQLVars.Comments, page)
	return &model.AnalysisCommentList{
		Nodes:    nodes,
		PageInfo: pageInfo,
	}, nil
}

func (r *findingResolver) AnalysisTrail(ctx context.Context, obj *model.Finding) (*model.AnalysisTrail, error) {
	return r.vulnerabilities.GetAnalysisTrailForImage(ctx, obj.ParentID, obj.ComponentID, obj.VulnerabilityID)
}

func (r *imageDetailsResolver) Findings(ctx context.Context, obj *model.ImageDetails, offset *int, limit *int, orderBy *model.OrderBy) (*model.FindingList, error) {
	if obj.ProjectID == "" {
		return &model.FindingList{
			Nodes: []*model.Finding{},
			PageInfo: model.PageInfo{
				HasNextPage:     false,
				HasPreviousPage: false,
				TotalCount:      0,
			},
		}, nil
	}
	findings, err := r.vulnerabilities.GetFindingsForImageByProjectID(ctx, obj.ProjectID, true)
	if err != nil {
		return nil, err
	}

	if orderBy != nil {
		switch orderBy.Field {
		case model.OrderByFieldName:
			model.SortWith(findings, func(a, b *model.Finding) bool {
				return model.Compare(a.VulnerabilityID, b.VulnerabilityID, orderBy.Direction)
			})
		case model.OrderByFieldSeverity:
			model.SortWith(findings, func(a, b *model.Finding) bool {
				severityToScore := map[string]int{
					"CRITICAL":   5,
					"HIGH":       4,
					"MEDIUM":     3,
					"LOW":        2,
					"UNASSIGNED": 1,
				}

				if orderBy.Direction == model.SortOrderAsc {
					return severityToScore[a.Severity] < severityToScore[b.Severity]
				}

				return severityToScore[a.Severity] > severityToScore[b.Severity]
			})
		case model.OrderByFieldPackageURL:
			model.SortWith(findings, func(a, b *model.Finding) bool {
				return model.Compare(a.PackageURL, b.PackageURL, orderBy.Direction)
			})
		case model.OrderByFieldState:
			model.SortWith(findings, func(a, b *model.Finding) bool {
				return model.Compare(a.State, b.State, orderBy.Direction)
			})
		}
	}

	pagination := model.NewPagination(offset, limit)
	findings, pageInfo := model.PaginatedSlice(findings, pagination)

	return &model.FindingList{
		Nodes:    findings,
		PageInfo: pageInfo,
	}, nil
}

func (r *imageDetailsResolver) WorkloadReferences(ctx context.Context, obj *model.ImageDetails) ([]model.Workload, error) {
	ret := make([]model.Workload, 0, len(obj.GQLVars.WorkloadReferences))
	for _, ref := range obj.GQLVars.WorkloadReferences {
		var workload model.Workload
		var err error

		switch ref.WorkloadType {
		case "app":
			workload, err = r.k8sClient.App(ctx, ref.Name, ref.Team, ref.Environment)
		case "job":
			workload, err = r.k8sClient.NaisJob(ctx, ref.Name, ref.Team, ref.Environment)
		default:
			err = fmt.Errorf("unknown workload type: %s", ref.WorkloadType)
		}
		if err != nil {
			return nil, err
		}
		if workload == nil {
			continue
		}

		ret = append(ret, workload)
	}
	return ret, nil
}

func (r *mutationResolver) SuppressFinding(ctx context.Context, analysisState string, comment string, componentID string, projectID string, vulnerabilityID string, suppressedBy string, suppress bool, team slug.Slug) (*model.AnalysisTrail, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireTeamMembership(actor, team)
	if err != nil {
		return nil, err
	}

	options := []string{
		"IN_TRIAGE",
		"RESOLVED",
		"FALSE_POSITIVE",
		"NOT_AFFECTED",
	}

	var valid bool
	for _, o := range options {
		if analysisState == o {
			valid = true
			break
		}
	}

	if _, err := uuid.Parse(componentID); err != nil {
		return nil, fmt.Errorf("invalid component ID: %s", componentID)
	}
	if _, err := uuid.Parse(projectID); err != nil {
		return nil, fmt.Errorf("invalid project ID: %s", projectID)
	}
	if _, err := uuid.Parse(vulnerabilityID); err != nil {
		return nil, fmt.Errorf("invalid vulnerability ID: %s", vulnerabilityID)
	}

	if !valid {
		return nil, fmt.Errorf("invalid analysis state: %s", analysisState)
	}

	trail, err := r.vulnerabilities.SuppressFinding(ctx, analysisState, comment, componentID, projectID, vulnerabilityID, suppressedBy, suppress)
	if err != nil {
		return nil, err
	}

	return trail, nil
}

func (r *Resolver) AnalysisTrail() gengql.AnalysisTrailResolver { return &analysisTrailResolver{r} }

func (r *Resolver) Finding() gengql.FindingResolver { return &findingResolver{r} }

func (r *Resolver) ImageDetails() gengql.ImageDetailsResolver { return &imageDetailsResolver{r} }

type (
	analysisTrailResolver struct{ *Resolver }
	findingResolver       struct{ *Resolver }
	imageDetailsResolver  struct{ *Resolver }
)
