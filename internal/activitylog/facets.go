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

	// From/To narrow the outer WHERE scope (which rows are candidates).
	// FilterFrom/FilterTo narrow the inner COUNT(*) FILTER (which rows count as "selected").
	// Both are set to the same time range: the user's time filter applies to both.
	activityTypeRows, err := q.FacetsForActivityTypes(ctx, activitylogsql.FacetsForActivityTypesParams{
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

	// Team facets are only computed at tenant scope (when no team is specified),
	// since a single-team scope always produces a trivial single-entry result.
	// This avoids a multiplicative cardinality explosion (teams × resource_types × actions × environments).
	var teamRows []*activitylogsql.FacetsForTeamsRow
	if scope == nil || scope.TeamSlug == nil {
		teamRows, err = q.FacetsForTeams(ctx, activitylogsql.FacetsForTeamsParams{
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
	}

	return buildFacets(activityTypeRows, teamRows), nil
}

func scopeField(scope *ActivityLogScope, fn func(*ActivityLogScope) *string) *string {
	if scope == nil {
		return nil
	}
	return fn(scope)
}

func buildFacets(activityTypeRows []*activitylogsql.FacetsForActivityTypesRow, teamRows []*activitylogsql.FacetsForTeamsRow) *ActivityLogFacets {
	activityTypeCounts := map[ActivityLogActivityType]int{}
	resourceTypeCounts := map[ActivityLogEntryResourceType]int{}
	environmentCounts := map[string]int{}
	teamCounts := map[string]int{}

	for _, row := range activityTypeRows {
		// Seed with 0 to ensure all values that exist in this scope are present
		rt := ActivityLogEntryResourceType(row.ResourceType)
		if _, ok := resourceTypeCounts[rt]; !ok {
			resourceTypeCounts[rt] = 0
		}

		if row.Environment != "" {
			if _, ok := environmentCounts[row.Environment]; !ok {
				environmentCounts[row.Environment] = 0
			}
		}

		for _, at := range LookupActivityTypes(row.ResourceType, row.Action) {
			if _, ok := activityTypeCounts[at]; !ok {
				activityTypeCounts[at] = 0
			}
		}

		filteredCount := int(row.FilteredCount)
		resourceTypeCounts[rt] += filteredCount

		if row.Environment != "" {
			environmentCounts[row.Environment] += filteredCount
		}

		for _, at := range LookupActivityTypes(row.ResourceType, row.Action) {
			activityTypeCounts[at] += filteredCount
		}
	}

	for _, row := range teamRows {
		if row.TeamSlug == nil {
			continue
		}
		teamSlug := row.TeamSlug.String()
		if teamSlug == "" {
			continue
		}
		if _, ok := teamCounts[teamSlug]; !ok {
			teamCounts[teamSlug] = 0
		}
		teamCounts[teamSlug] += int(row.FilteredCount)
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
