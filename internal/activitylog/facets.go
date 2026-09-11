package activitylog

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/nais/api/internal/activitylog/activitylogsql"
	"github.com/nais/api/internal/graph/model"
)

func ComputeFacets(ctx context.Context, scope *ActivityLogScope, filter *ActivityLogFilter) (*ActivityLogFacets, error) {
	q := db(ctx)
	filterValues := withFilters(filter)
	resourceTypes := withResourceTypes(filter)
	environments := withEnvironments(filter)
	from := withFrom(filter)
	to := withTo(filter)

	if scope != nil && scope.TenantWide {
		rows, err := q.FacetsForTenantActivityTypes(ctx, activitylogsql.FacetsForTenantActivityTypesParams{
			From:                from,
			To:                  to,
			Filter:              filterValues,
			FilterResourceTypes: resourceTypes,
			FilterEnvironments:  environments,
			FilterFrom:          from,
			FilterTo:            to,
		})
		if err != nil {
			return nil, err
		}

		return buildFacets(rows, func(row *activitylogsql.FacetsForTenantActivityTypesRow) facetValues {
			return facetValues{
				resourceType:  row.ResourceType,
				action:        row.Action,
				environment:   row.Environment,
				filteredCount: row.FilteredCount,
			}
		}), nil
	}

	if scope == nil {
		return nil, fmt.Errorf("activity log facet scope is required")
	}

	switch {
	case scope.EnvironmentName != nil && scope.TeamSlug != nil && scope.ResourceType != nil && scope.ResourceName != nil:
		rows, err := q.FacetsForResourceTeamAndEnvironment(ctx, activitylogsql.FacetsForResourceTeamAndEnvironmentParams{
			Filter:              filterValues,
			FilterResourceTypes: resourceTypes,
			FilterEnvironments:  environments,
			FilterFrom:          from,
			FilterTo:            to,
			ResourceType:        *scope.ResourceType,
			ResourceName:        *scope.ResourceName,
			TeamSlug:            scope.TeamSlug,
			EnvironmentName:     scope.EnvironmentName,
			From:                from,
			To:                  to,
		})
		if err != nil {
			return nil, err
		}

		return buildFacets(rows, facetValuesForResourceTeamAndEnvironment), nil

	case scope.MatchNullTeam && scope.ResourceType != nil && scope.ResourceName != nil:
		if scope.TeamSlug == nil {
			rows, err := q.FacetsForResourceWithoutTeam(ctx, activitylogsql.FacetsForResourceWithoutTeamParams{
				Filter:              filterValues,
				FilterResourceTypes: resourceTypes,
				FilterEnvironments:  environments,
				FilterFrom:          from,
				FilterTo:            to,
				ResourceType:        *scope.ResourceType,
				ResourceName:        *scope.ResourceName,
				From:                from,
				To:                  to,
			})
			if err != nil {
				return nil, err
			}

			return buildFacets(rows, facetValuesForResourceWithoutTeam), nil
		}

		rows, err := q.FacetsForResourceAndTeam(ctx, activitylogsql.FacetsForResourceAndTeamParams{
			Filter:              filterValues,
			FilterResourceTypes: resourceTypes,
			FilterEnvironments:  environments,
			FilterFrom:          from,
			FilterTo:            to,
			ResourceType:        *scope.ResourceType,
			ResourceName:        *scope.ResourceName,
			TeamSlug:            scope.TeamSlug,
			From:                from,
			To:                  to,
		})
		if err != nil {
			return nil, err
		}

		return buildFacets(rows, facetValuesForResourceAndTeam), nil

	case scope.ResourceType != nil && scope.ResourceName != nil:
		rows, err := q.FacetsForResource(ctx, activitylogsql.FacetsForResourceParams{
			Filter:              filterValues,
			FilterResourceTypes: resourceTypes,
			FilterEnvironments:  environments,
			FilterFrom:          from,
			FilterTo:            to,
			ResourceType:        *scope.ResourceType,
			ResourceName:        *scope.ResourceName,
			From:                from,
			To:                  to,
		})
		if err != nil {
			return nil, err
		}

		return buildFacets(rows, facetValuesForResource), nil

	case scope.TeamSlug != nil:
		rows, err := q.FacetsForTeam(ctx, activitylogsql.FacetsForTeamParams{
			Filter:              filterValues,
			FilterResourceTypes: resourceTypes,
			FilterEnvironments:  environments,
			FilterFrom:          from,
			FilterTo:            to,
			TeamSlug:            scope.TeamSlug,
			From:                from,
			To:                  to,
		})
		if err != nil {
			return nil, err
		}

		return buildFacets(rows, facetValuesForTeam), nil

	default:
		return nil, fmt.Errorf("unsupported activity log facet scope")
	}
}

func facetValuesForTeam(row *activitylogsql.FacetsForTeamRow) facetValues {
	return facetValues{resourceType: row.ResourceType, action: row.Action, environment: row.Environment, filteredCount: row.FilteredCount}
}

func facetValuesForResource(row *activitylogsql.FacetsForResourceRow) facetValues {
	return facetValues{resourceType: row.ResourceType, action: row.Action, environment: row.Environment, filteredCount: row.FilteredCount}
}

func facetValuesForResourceAndTeam(row *activitylogsql.FacetsForResourceAndTeamRow) facetValues {
	return facetValues{resourceType: row.ResourceType, action: row.Action, environment: row.Environment, filteredCount: row.FilteredCount}
}

func facetValuesForResourceWithoutTeam(row *activitylogsql.FacetsForResourceWithoutTeamRow) facetValues {
	return facetValues{resourceType: row.ResourceType, action: row.Action, environment: row.Environment, filteredCount: row.FilteredCount}
}

func facetValuesForResourceTeamAndEnvironment(row *activitylogsql.FacetsForResourceTeamAndEnvironmentRow) facetValues {
	return facetValues{resourceType: row.ResourceType, action: row.Action, environment: row.Environment, filteredCount: row.FilteredCount}
}

type facetValues struct {
	resourceType  string
	action        string
	environment   string
	filteredCount int64
}

func buildFacets[T any](activityTypeRows []*T, values func(*T) facetValues) *ActivityLogFacets {
	activityTypeCounts := map[ActivityLogActivityType]int{}
	resourceTypeCounts := map[ActivityLogEntryResourceType]int{}
	environmentCounts := map[string]int{}

	for _, row := range activityTypeRows {
		row := values(row)

		// Seed with 0 to ensure all values that exist in this scope are present
		rt := ActivityLogEntryResourceType(row.resourceType)
		if _, ok := resourceTypeCounts[rt]; !ok {
			resourceTypeCounts[rt] = 0
		}

		if row.environment != "" {
			if _, ok := environmentCounts[row.environment]; !ok {
				environmentCounts[row.environment] = 0
			}
		}

		for _, at := range LookupActivityTypes(row.resourceType, row.action) {
			if _, ok := activityTypeCounts[at]; !ok {
				activityTypeCounts[at] = 0
			}
		}

		filteredCount := int(row.filteredCount)
		resourceTypeCounts[rt] += filteredCount

		if row.environment != "" {
			environmentCounts[row.environment] += filteredCount
		}

		for _, at := range LookupActivityTypes(row.resourceType, row.action) {
			activityTypeCounts[at] += filteredCount
		}
	}

	return assembleFacets(activityTypeCounts, resourceTypeCounts, environmentCounts)
}

func assembleFacets(activityTypeCounts map[ActivityLogActivityType]int, resourceTypeCounts map[ActivityLogEntryResourceType]int, environmentCounts map[string]int) *ActivityLogFacets {
	facets := &ActivityLogFacets{
		ActivityTypes: make([]ActivityLogActivityTypeFacetItem, 0, len(activityTypeCounts)),
		ResourceTypes: make([]ActivityLogResourceTypeFacetItem, 0, len(resourceTypeCounts)),
		Environments:  make([]model.StringFacetItem, 0, len(environmentCounts)),
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

	// Sort alphabetically for stable ordering (items don't jump around when filters change)
	slices.SortFunc(facets.ActivityTypes, func(a, b ActivityLogActivityTypeFacetItem) int {
		return strings.Compare(string(a.ActivityType), string(b.ActivityType))
	})

	slices.SortFunc(facets.ResourceTypes, func(a, b ActivityLogResourceTypeFacetItem) int {
		return strings.Compare(string(a.ResourceType), string(b.ResourceType))
	})

	model.SortStringFacetItems(facets.Environments)

	return facets
}
