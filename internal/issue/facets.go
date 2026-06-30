package issue

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/issue/issuesql"
	"github.com/nais/api/internal/slug"
)

func ComputeFacets(ctx context.Context, teamSlug slug.Slug, filter *IssueFilter) (*IssueFacets, error) {
	params := issuesql.FacetsForIssuesParams{
		Team: teamSlug.String(),
	}

	if filter != nil {
		params.Env = filter.Environments
		params.ResourceType = (*string)(filter.ResourceType)
		params.IssueType = (*string)(filter.IssueType)
		params.ResourceName = filter.ResourceName
		if filter.Severity != nil {
			params.Severity = new(issuesql.SeverityLevel(*filter.Severity))
		}
	}

	rows, err := db(ctx).FacetsForIssues(ctx, params)
	if err != nil {
		return nil, err
	}

	return buildFacets(rows), nil
}

func buildFacets(rows []*issuesql.FacetsForIssuesRow) *IssueFacets {
	severityCounts := map[Severity]int{}
	resourceTypeCounts := map[ResourceType]int{}
	environmentCounts := map[string]int{}
	issueTypeCounts := map[IssueType]int{}

	for _, row := range rows {
		sev := Severity(row.Severity)
		rt := ResourceType(row.ResourceType)
		it := IssueType(row.IssueType)

		if _, ok := severityCounts[sev]; !ok {
			severityCounts[sev] = 0
		}
		if _, ok := resourceTypeCounts[rt]; !ok {
			resourceTypeCounts[rt] = 0
		}
		if _, ok := environmentCounts[row.Env]; !ok {
			environmentCounts[row.Env] = 0
		}
		if _, ok := issueTypeCounts[it]; !ok {
			issueTypeCounts[it] = 0
		}

		filtered := int(row.FilteredCount)
		severityCounts[sev] += filtered
		resourceTypeCounts[rt] += filtered
		environmentCounts[row.Env] += filtered
		issueTypeCounts[it] += filtered
	}

	severities := make([]SeverityFacetItem, 0, len(severityCounts))
	for sev, count := range severityCounts {
		severities = append(severities, SeverityFacetItem{Severity: sev, Count: count})
	}
	slices.SortFunc(severities, func(a, b SeverityFacetItem) int {
		return strings.Compare(a.Severity.String(), b.Severity.String())
	})

	resourceTypes := make([]ResourceTypeFacetItem, 0, len(resourceTypeCounts))
	for rt, count := range resourceTypeCounts {
		resourceTypes = append(resourceTypes, ResourceTypeFacetItem{ResourceType: rt, Count: count})
	}
	slices.SortFunc(resourceTypes, func(a, b ResourceTypeFacetItem) int {
		return strings.Compare(a.ResourceType.String(), b.ResourceType.String())
	})

	environments := make([]model.StringFacetItem, 0, len(environmentCounts))
	for env, count := range environmentCounts {
		environments = append(environments, model.StringFacetItem{Value: env, Count: count})
	}
	model.SortStringFacetItems(environments)

	issueTypes := make([]IssueTypeFacetItem, 0, len(issueTypeCounts))
	for it, count := range issueTypeCounts {
		issueTypes = append(issueTypes, IssueTypeFacetItem{IssueType: it, Count: count})
	}
	slices.SortFunc(issueTypes, func(a, b IssueTypeFacetItem) int {
		return strings.Compare(a.IssueType.String(), b.IssueType.String())
	})

	return &IssueFacets{
		Environments:  environments,
		Severities:    severities,
		ResourceTypes: resourceTypes,
		IssueTypes:    issueTypes,
	}
}
