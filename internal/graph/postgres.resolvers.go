package graph

import (
	"context"
	"time"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/environment"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/issue"
	"github.com/nais/api/internal/persistence/postgres"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
	liberatorv1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
)

func (r *applicationResolver) Postgreses(ctx context.Context, obj *application.Application, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*postgres.WorkloadPostgres], error) {
	var uses []liberatorv1.PostgresUse
	if obj.Spec != nil && obj.Spec.Uses != nil {
		uses = obj.Spec.Uses.Postgres
	}
	return workloadPostgresConnectionForWorkload(ctx, obj.TeamSlug, obj.EnvironmentName, uses, first, after, last, before)
}

func (r *availableValuesResolver) PostgresMajorVersions(ctx context.Context, obj *environment.AvailableValues) (*environment.AvailableValueOptions, error) {
	return postgres.AvailableMajorVersions(ctx, obj.EnvironmentName)
}

func (r *availableValuesResolver) PostgresExtensions(ctx context.Context, obj *environment.AvailableValues) (*environment.AvailableValueOptions, error) {
	return postgres.AvailableExtensions(ctx, obj.EnvironmentName)
}

func (r *jobResolver) Postgreses(ctx context.Context, obj *job.Job, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*postgres.WorkloadPostgres], error) {
	var uses []liberatorv1.PostgresUse
	if obj.Spec != nil && obj.Spec.Uses != nil {
		uses = obj.Spec.Uses.Postgres
	}
	return workloadPostgresConnectionForWorkload(ctx, obj.TeamSlug, obj.EnvironmentName, uses, first, after, last, before)
}

func (r *mutationResolver) CreatePostgresAccess(ctx context.Context, input postgres.CreatePostgresAccessInput) (*postgres.CreatePostgresAccessPayload, error) {
	if err := authz.CanGrantPostgresAccess(ctx, input.TeamSlug); err != nil {
		return nil, err
	}

	return postgres.CreatePostgresAccess(ctx, input)
}

func (r *mutationResolver) CreatePostgres(ctx context.Context, input postgres.CreatePostgresInput) (*postgres.CreatePostgresPayload, error) {
	if err := authz.CanCreatePostgres(ctx, input.TeamSlug); err != nil {
		return nil, err
	}
	return postgres.Create(ctx, input)
}

func (r *mutationResolver) UpdatePostgres(ctx context.Context, input postgres.UpdatePostgresInput) (*postgres.UpdatePostgresPayload, error) {
	if err := authz.CanUpdatePostgres(ctx, input.TeamSlug); err != nil {
		return nil, err
	}
	return postgres.Update(ctx, input)
}

func (r *mutationResolver) DeletePostgres(ctx context.Context, input postgres.DeletePostgresInput) (*postgres.DeletePostgresPayload, error) {
	if err := authz.CanDeletePostgres(ctx, input.TeamSlug); err != nil {
		return nil, err
	}
	return postgres.DeletePostgres(ctx, input)
}

func (r *postgresResolver) Team(ctx context.Context, obj *postgres.Postgres) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *postgresResolver) TeamEnvironment(ctx context.Context, obj *postgres.Postgres) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *postgresResolver) State(ctx context.Context, obj *postgres.Postgres) (postgres.PostgresBranchState, error) {
	state, _, err := postgres.PhysicalStatus(ctx, obj)
	return state, err
}

func (r *postgresResolver) Configuration(ctx context.Context, obj *postgres.Postgres) (*postgres.PostgresConfiguration, error) {
	_, configuration, err := postgres.PhysicalStatus(ctx, obj)
	return configuration, err
}

func (r *postgresResolver) EarliestRestoreTime(ctx context.Context, obj *postgres.Postgres) (*time.Time, error) {
	return postgres.PostgresEarliestRestoreTime(ctx, obj)
}

func (r *postgresResolver) Issues(ctx context.Context, obj *postgres.Postgres, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *issue.IssueOrder, filter *issue.ResourceIssueFilter) (*issue.IssueConnection, error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}
	scope := &issue.IssueScope{
		ResourceName: obj.Name,
		ResourceType: issue.ResourceTypePostgres,
		Env:          obj.EnvironmentName,
	}
	var f *issue.IssueFilter
	if filter != nil {
		f = &issue.IssueFilter{ResourceIssueFilter: *filter}
	}
	return issue.ListIssues(ctx, obj.TeamSlug, page, orderBy, scope, f)
}

func (r *postgresResolver) Workloads(ctx context.Context, obj *postgres.Postgres, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*postgres.PostgresWorkload], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}
	workloads := postgres.WorkloadsForPostgres(ctx, obj)
	return pagination.NewConnection(pagination.Slice(workloads, page), page, len(workloads)), nil
}

func (r *postgresAccessResolver) Team(ctx context.Context, obj *postgres.PostgresAccess) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *postgresAccessResolver) TeamEnvironment(ctx context.Context, obj *postgres.PostgresAccess) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *postgresAccessResolver) Postgres(ctx context.Context, obj *postgres.PostgresAccess) (*postgres.Postgres, error) {
	if obj.PostgresName != "" {
		return postgres.GetPostgres(ctx, obj.TeamSlug, obj.EnvironmentName, obj.PostgresName)
	}
	return postgres.GetPostgresByPhysicalName(ctx, obj.TeamSlug, obj.EnvironmentName, obj.PostgresBranchName)
}

func (r *postgresAccessResolver) Connection(ctx context.Context, obj *postgres.PostgresAccess) (*postgres.PostgresAccessConnectionDetails, error) {
	return postgres.GetPostgresAccessConnection(ctx, postgres.PostgresAccessConnectionInput{
		Name: obj.Name, TeamSlug: obj.TeamSlug, EnvironmentName: obj.EnvironmentName,
	})
}

func (r *postgresConnectionResolver) Facets(ctx context.Context, obj *pagination.FacetableConnection[*postgres.Postgres, *postgres.TeamPostgresFilter]) (*postgres.PostgresFacets, error) {
	filtered := make([]*postgres.Postgres, 0, len(obj.GetAllItems()))
	for _, pg := range obj.GetAllItems() {
		if obj.GetFilter().Matches(pg) {
			filtered = append(filtered, pg)
		}
	}
	return &postgres.PostgresFacets{AllInstances: obj.GetAllItems(), FilteredInstances: filtered}, nil
}

func (r *teamResolver) Postgreses(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, filter *postgres.TeamPostgresFilter) (*pagination.FacetableConnection[*postgres.Postgres, *postgres.TeamPostgresFilter], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}
	return postgres.ListPostgresForTeam(ctx, obj.Slug, page, filter), nil
}

func (r *teamEnvironmentResolver) Postgres(ctx context.Context, obj *team.TeamEnvironment, name string) (*postgres.Postgres, error) {
	return postgres.GetPostgres(ctx, obj.TeamSlug, obj.EnvironmentName, name)
}

func (r *teamEnvironmentResolver) PostgresAccess(ctx context.Context, obj *team.TeamEnvironment, name string) (*postgres.PostgresAccess, error) {
	return postgres.GetPostgresAccess(ctx, name, obj.TeamSlug, obj.EnvironmentName)
}

func (r *Resolver) Postgres() gengql.PostgresResolver { return &postgresResolver{r} }

func (r *Resolver) PostgresAccess() gengql.PostgresAccessResolver { return &postgresAccessResolver{r} }

func (r *Resolver) PostgresConnection() gengql.PostgresConnectionResolver {
	return &postgresConnectionResolver{r}
}

type (
	postgresResolver           struct{ *Resolver }
	postgresAccessResolver     struct{ *Resolver }
	postgresConnectionResolver struct{ *Resolver }
)
