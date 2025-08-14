package activitylog

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/nais/api/internal/activitylog/activitylogsql"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/environmentmapper"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"k8s.io/utils/ptr"
)

type CreateInput struct {
	Action       ActivityLogEntryAction
	Actor        authz.AuthenticatedUser
	ResourceType ActivityLogEntryResourceType
	ResourceName string

	Data            any        // optional
	EnvironmentName *string    // optional
	TeamSlug        *slug.Slug // optional
}

func MarshalData(input CreateInput) ([]byte, error) {
	if input.Data == nil {
		return nil, nil
	}

	bytes, err := json.Marshal(input.Data)
	if err != nil {
		return nil, fmt.Errorf("marshaling data: %w", err)
	}

	return bytes, nil
}

func Create(ctx context.Context, input CreateInput) error {
	q := db(ctx)

	data, err := MarshalData(input)
	if err != nil {
		return err
	}

	var environmentName *string
	if input.EnvironmentName != nil {
		environmentName = ptr.To(environmentmapper.EnvironmentName(*input.EnvironmentName))
	}

	return q.Create(ctx, activitylogsql.CreateParams{
		Action:          string(input.Action),
		Actor:           input.Actor.Identity(),
		Data:            data,
		EnvironmentName: environmentName,
		ResourceName:    input.ResourceName,
		ResourceType:    string(input.ResourceType),
		TeamSlug:        input.TeamSlug,
	})
}

func Get(ctx context.Context, uid uuid.UUID) (ActivityLogEntry, error) {
	return fromContext(ctx).activityLogLoader.Load(ctx, uid)
}

func GetByIdent(ctx context.Context, id ident.Ident) (ActivityLogEntry, error) {
	uid, err := parseIdent(id)
	if err != nil {
		return nil, err
	}
	return Get(ctx, uid)
}

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination, filter *ActivityLogFilter) (*ActivityLogEntryConnection, error) {
	q := db(ctx)

	ret, err := q.ListForTeam(ctx, activitylogsql.ListForTeamParams{
		TeamSlug: ptr.To(teamSlug),
		Offset:   page.Offset(),
		Limit:    page.Limit(),
		Filter:   withFilters(filter),
	})
	if err != nil {
		return nil, err
	}

	var total int64
	if len(ret) > 0 {
		total = ret[0].TotalCount
	}
	return pagination.NewConvertConnectionWithError(ret, page, total, func(from *activitylogsql.ListForTeamRow) (ActivityLogEntry, error) {
		return toGraphActivityLogEntry(&from.ActivityLogCombinedView)
	})
}

func ListForResource(ctx context.Context, resourceType ActivityLogEntryResourceType, resourceName string, page *pagination.Pagination, filter *ActivityLogFilter) (*ActivityLogEntryConnection, error) {
	q := db(ctx)

	ret, err := q.ListForResource(ctx, activitylogsql.ListForResourceParams{
		ResourceType: string(resourceType),
		ResourceName: resourceName,
		Offset:       page.Offset(),
		Limit:        page.Limit(),
		Filter:       withFilters(filter),
	})
	if err != nil {
		return nil, err
	}

	var total int64
	if len(ret) > 0 {
		total = ret[0].TotalCount
	}
	return pagination.NewConvertConnectionWithError(ret, page, total, func(from *activitylogsql.ListForResourceRow) (ActivityLogEntry, error) {
		return toGraphActivityLogEntry(&from.ActivityLogCombinedView)
	})
}

func ListForResourceTeamAndEnvironment(ctx context.Context, resourceType ActivityLogEntryResourceType, teamSlug slug.Slug, resourceName, environmentName string, page *pagination.Pagination, filter *ActivityLogFilter) (*ActivityLogEntryConnection, error) {
	q := db(ctx)

	ret, err := q.ListForResourceTeamAndEnvironment(ctx, activitylogsql.ListForResourceTeamAndEnvironmentParams{
		ResourceType:    string(resourceType),
		ResourceName:    resourceName,
		EnvironmentName: ptr.To(environmentName),
		TeamSlug:        ptr.To(teamSlug),
		Offset:          page.Offset(),
		Limit:           page.Limit(),
		Filter:          withFilters(filter),
	})
	if err != nil {
		return nil, err
	}

	var total int64
	if len(ret) > 0 {
		total = ret[0].TotalCount
	}
	return pagination.NewConvertConnectionWithError(ret, page, total, func(from *activitylogsql.ListForResourceTeamAndEnvironmentRow) (ActivityLogEntry, error) {
		return toGraphActivityLogEntry(&from.ActivityLogCombinedView)
	})
}

func toGraphActivityLogEntry(row *activitylogsql.ActivityLogCombinedView) (ActivityLogEntry, error) {
	titler := cases.Title(language.English)
	entry := GenericActivityLogEntry{
		Action:          ActivityLogEntryAction(row.Action),
		Actor:           row.Actor,
		CreatedAt:       row.CreatedAt.Time,
		EnvironmentName: row.Environment,
		Message:         titler.String(row.Action) + " " + titler.String(row.ResourceType),
		ResourceType:    ActivityLogEntryResourceType(row.ResourceType),
		ResourceName:    row.ResourceName,
		TeamSlug:        row.TeamSlug,
		UUID:            row.ID,
		Data:            row.Data,
	}

	transformer, ok := knownTransformersForAction[ActivityLogEntryAction(row.Action)]
	if ok {
		return transformer(entry)
	}

	transformer, ok = knownTransformers[ActivityLogEntryResourceType(row.ResourceType)]
	if !ok {
		return nil, fmt.Errorf("no transformer registered for activity log resource type: %q", row.ResourceType)
	}

	return transformer(entry)
}
