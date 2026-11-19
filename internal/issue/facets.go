package issue

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/issue/issuesql"
	"github.com/nais/api/internal/slug"
)

const teamFacetLimit = 25

func ComputeFacets(ctx context.Context, connection *IssueConnection) (*IssueFacets, error) {
	params := issuesql.FacetsForIssuesParams{
		Team:      connection.teamSlug.String(),
		TeamSlugs: connection.teamSlugs,
	}
	scope, filter := connection.scope, connection.filter

	if scope != nil {
		params.ScopeResourceName = &scope.ResourceName
		params.ScopeResourceType = (*string)(&scope.ResourceType)
		params.ScopeEnv = &scope.Env
	}

	if filter != nil {
		if scope == nil {
			if len(filter.Environments) > 0 {
				params.Env = filter.Environments
			}
			params.FilterResourceType = (*string)(filter.ResourceType)
			params.FilterResourceName = filter.ResourceName
		}
		params.IssueType = (*string)(filter.IssueType)
		if filter.Severity != nil {
			params.Severity = new(issuesql.SeverityLevel(*filter.Severity))
		}
	}

	params.LegacyScopeResourceName = legacyPostgresIssueName(params.ScopeResourceName)
	params.LegacyFilterResourceName = legacyPostgresIssueName(params.FilterResourceName)
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
	teamCounts := map[slug.Slug]int{}

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
		if filtered > 0 && row.TeamExists {
			teamCounts[slug.Slug(row.Team)] += filtered
		}
		severityCounts[sev] += filtered
		resourceTypeCounts[rt] += filtered
		environmentCounts[row.Env] += filtered
		issueTypeCounts[it] += filtered
	}

	severities := make([]IssueSeverityFacetItem, 0, len(severityCounts))
	for sev, count := range severityCounts {
		severities = append(severities, IssueSeverityFacetItem{Severity: sev, Count: count})
	}
	slices.SortFunc(severities, func(a, b IssueSeverityFacetItem) int {
		return strings.Compare(a.Severity.String(), b.Severity.String())
	})

	resourceTypes := make([]IssueResourceTypeFacetItem, 0, len(resourceTypeCounts))
	for rt, count := range resourceTypeCounts {
		resourceTypes = append(resourceTypes, IssueResourceTypeFacetItem{ResourceType: rt, Count: count})
	}
	slices.SortFunc(resourceTypes, func(a, b IssueResourceTypeFacetItem) int {
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

	teams := make([]IssueTeamFacetItem, 0, len(teamCounts))
	for team, count := range teamCounts {
		teams = append(teams, IssueTeamFacetItem{TeamSlug: team, Count: count})
	}
	slices.SortFunc(teams, func(a, b IssueTeamFacetItem) int {
		if a.Count != b.Count {
			return b.Count - a.Count
		}
		return strings.Compare(a.TeamSlug.String(), b.TeamSlug.String())
	})
	teams = teams[:min(teamFacetLimit, len(teams))]

	return &IssueFacets{
		Teams:         teams,
		Environments:  environments,
		Severities:    severities,
		ResourceTypes: resourceTypes,
		IssueTypes:    issueTypes,
	}
}
