package activitylog

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/activitylog/activitylogsql"
	"github.com/nais/api/internal/graph/model"
)

func ComputeFacets(ctx context.Context, scope *ActivityLogScope, filter *ActivityLogFilter) (*ActivityLogFacets, error) {
	q := db(ctx)

	rows, err := q.Facets(ctx, activitylogsql.FacetsParams{
		TeamSlug:            scopeField(scope, func(s *ActivityLogScope) *string { return (*string)(s.TeamSlug) }),
		ResourceType:        scopeField(scope, func(s *ActivityLogScope) *string { return s.ResourceType }),
		ResourceName:        scopeField(scope, func(s *ActivityLogScope) *string { return s.ResourceName }),
		EnvironmentName:     scopeField(scope, func(s *ActivityLogScope) *string { return s.EnvironmentName }),
		From:                withFrom(filter),
		To:                  withTo(filter),
		Filter:              withFilters(filter),
		FilterResourceTypes: withResourceTypes(filter),
		FilterEnvironments:  withEnvironments(filter),
		FilterFrom:          withFrom(filter),
		FilterTo:            withTo(filter),
	})
	if err != nil {
		return nil, err
	}

	return buildFacets(rows), nil
}

func scopeField(scope *ActivityLogScope, fn func(*ActivityLogScope) *string) *string {
	if scope == nil {
		return nil
	}
	return fn(scope)
}

func buildFacets(rows []*activitylogsql.FacetsRow) *ActivityLogFacets {
	activityTypeCounts := map[ActivityLogActivityType]int{}
	resourceTypeCounts := map[ActivityLogEntryResourceType]int{}
	environmentCounts := map[string]int{}
	teamCounts := map[string]int{}

	for _, row := range rows {
		// Seed with total_count to ensure all values that exist in this scope are present
		rt := ActivityLogEntryResourceType(row.ResourceType)
		if _, ok := resourceTypeCounts[rt]; !ok {
			resourceTypeCounts[rt] = 0
		}

		if row.Environment != "" {
			if _, ok := environmentCounts[row.Environment]; !ok {
				environmentCounts[row.Environment] = 0
			}
		}

		if row.TeamSlug != nil {
			teamSlug := row.TeamSlug.String()
			if _, ok := teamCounts[teamSlug]; !ok {
				teamCounts[teamSlug] = 0
			}
		}

		for _, at := range LookupActivityTypes(row.ResourceType, row.Action) {
			if _, ok := activityTypeCounts[at]; !ok {
				activityTypeCounts[at] = 0
			}
		}

		// Now add the filtered counts
		filteredCount := int(row.FilteredCount)
		resourceTypeCounts[rt] += filteredCount

		if row.Environment != "" {
			environmentCounts[row.Environment] += filteredCount
		}

		if row.TeamSlug != nil {
			teamCounts[row.TeamSlug.String()] += filteredCount
		}

		for _, at := range LookupActivityTypes(row.ResourceType, row.Action) {
			activityTypeCounts[at] += filteredCount
		}
	}

	return assembleFacets(activityTypeCounts, resourceTypeCounts, environmentCounts, teamCounts)
}

func assembleFacets(activityTypeCounts map[ActivityLogActivityType]int, resourceTypeCounts map[ActivityLogEntryResourceType]int, environmentCounts map[string]int, teamCounts map[string]int) *ActivityLogFacets {
	facets := &ActivityLogFacets{
		ActivityTypes: make([]ActivityLogActivityTypeFacetItem, 0, len(activityTypeCounts)),
		ResourceTypes: make([]ActivityLogResourceTypeFacetItem, 0, len(resourceTypeCounts)),
		Environments:  make([]model.StringFacetItem, 0, len(environmentCounts)),
		Teams:         make([]model.StringFacetItem, 0, len(teamCounts)),
	}

	for at, count := range activityTypeCounts {
		facets.ActivityTypes = append(facets.ActivityTypes, ActivityLogActivityTypeFacetItem{
			ActivityType: at,
			Count:        count,
		})
	}

	for rt, count := range resourceTypeCounts {
		facets.ResourceTypes = append(facets.ResourceTypes, ActivityLogResourceTypeFacetItem{
			ResourceType: rt,
			Count:        count,
		})
	}

	for env, count := range environmentCounts {
		facets.Environments = append(facets.Environments, model.StringFacetItem{
			Value: env,
			Count: count,
		})
	}

	for teamSlug, count := range teamCounts {
		facets.Teams = append(facets.Teams, model.StringFacetItem{
			Value: teamSlug,
			Count: count,
		})
	}

	// Sort alphabetically for stable ordering (items don't jump around when filters change)
	slices.SortFunc(facets.ActivityTypes, func(a, b ActivityLogActivityTypeFacetItem) int {
		return strings.Compare(string(a.ActivityType), string(b.ActivityType))
	})

	slices.SortFunc(facets.ResourceTypes, func(a, b ActivityLogResourceTypeFacetItem) int {
		return strings.Compare(string(a.ResourceType), string(b.ResourceType))
	})

	model.SortStringFacetItems(facets.Environments)
	model.SortStringFacetItems(facets.Teams)

	return facets
}
