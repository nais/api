package graph

import (
	"context"
	"strings"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/persistence/opensearch"
	"github.com/nais/api/internal/persistence/valkey"
	"github.com/nais/api/internal/servicemaintenance"
	"github.com/nais/api/internal/thirdparty/aiven"
)

func (r *mutationResolver) StartValkeyMaintenance(ctx context.Context, input servicemaintenance.StartValkeyMaintenanceInput) (*servicemaintenance.StartValkeyMaintenancePayload, error) {
	if err := authz.CanStartServiceMaintenance(ctx, input.TeamSlug); err != nil {
		return nil, err
	}

	if err := servicemaintenance.StartValkeyMaintenance(ctx, input); err != nil {
		return nil, err
	}

	return &servicemaintenance.StartValkeyMaintenancePayload{
		Error: new(string),
	}, nil
}

func (r *mutationResolver) StartOpenSearchMaintenance(ctx context.Context, input servicemaintenance.StartOpenSearchMaintenanceInput) (*servicemaintenance.StartOpenSearchMaintenancePayload, error) {
	if err := authz.CanStartServiceMaintenance(ctx, input.TeamSlug); err != nil {
		return nil, err
	}

	if err := servicemaintenance.StartOpenSearchMaintenance(ctx, input); err != nil {
		return nil, err
	}

	return &servicemaintenance.StartOpenSearchMaintenancePayload{
		Error: new(string),
	}, nil
}

func (r *openSearchResolver) Maintenance(ctx context.Context, obj *opensearch.OpenSearch) (*servicemaintenance.OpenSearchMaintenance, error) {
	project, err := aiven.GetProject(ctx, obj.EnvironmentName)
	if err != nil {
		return nil, err
	}
	return &servicemaintenance.OpenSearchMaintenance{
		AivenProject: project.ID,
		ServiceName:  obj.FullyQualifiedName(),
	}, nil
}

func (r *openSearchMaintenanceResolver) Window(ctx context.Context, obj *servicemaintenance.OpenSearchMaintenance) (*servicemaintenance.MaintenanceWindow, error) {
	ret, err := servicemaintenance.GetAivenMaintenanceWindow(ctx, servicemaintenance.AivenDataLoaderKey{
		Project:     obj.AivenProject,
		ServiceName: obj.ServiceName,
	})

	if err != nil && strings.Contains(err.Error(), "404 ServiceGet") {
		// Since obj is OK, we know the service exists within Kubernetes, but it might not yet be
		// fully created in Aiven. In that case, return a default maintenance window.
		return &servicemaintenance.MaintenanceWindow{
			DayOfWeek: model.WeekdayMonday,
			TimeOfDay: "",
		}, nil
	}
	return ret, err
}

func (r *openSearchMaintenanceResolver) Updates(ctx context.Context, obj *servicemaintenance.OpenSearchMaintenance, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*servicemaintenance.OpenSearchMaintenanceUpdate], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	allUpdates, err := servicemaintenance.GetAivenMaintenanceUpdates[servicemaintenance.OpenSearchMaintenanceUpdate](ctx, servicemaintenance.AivenDataLoaderKey{
		Project:     obj.AivenProject,
		ServiceName: obj.ServiceName,
	})
	if err != nil {
		if strings.Contains(err.Error(), "404 ServiceGet") {
			// Since obj is OK, we know the service exists within Kubernetes, but it might not yet be
			// fully created in Aiven. In that case, return a default maintenance window.
			return pagination.EmptyConnection[*servicemaintenance.OpenSearchMaintenanceUpdate](), nil
		}
		return nil, err
	}

	return pagination.NewConnection(pagination.Slice(allUpdates, page), page, len(allUpdates)), nil
}

func (r *valkeyResolver) Maintenance(ctx context.Context, obj *valkey.Valkey) (*servicemaintenance.ValkeyMaintenance, error) {
	project, err := aiven.GetProject(ctx, obj.EnvironmentName)
	if err != nil {
		return nil, err
	}
	return &servicemaintenance.ValkeyMaintenance{
		AivenProject: project.ID,
		ServiceName:  obj.FullyQualifiedName(),
	}, nil
}

func (r *valkeyMaintenanceResolver) Window(ctx context.Context, obj *servicemaintenance.ValkeyMaintenance) (*servicemaintenance.MaintenanceWindow, error) {
	ret, err := servicemaintenance.GetAivenMaintenanceWindow(ctx, servicemaintenance.AivenDataLoaderKey{
		Project:     obj.AivenProject,
		ServiceName: obj.ServiceName,
	})

	if err != nil && strings.Contains(err.Error(), "404 ServiceGet") {
		// Since obj is OK, we know the service exists within Kubernetes, but it might not yet be
		// fully created in Aiven. In that case, return a default maintenance window.
		return &servicemaintenance.MaintenanceWindow{
			DayOfWeek: model.WeekdayMonday,
			TimeOfDay: "",
		}, nil
	}

	return ret, err
}

func (r *valkeyMaintenanceResolver) Updates(ctx context.Context, obj *servicemaintenance.ValkeyMaintenance, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*servicemaintenance.ValkeyMaintenanceUpdate], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	allUpdates, err := servicemaintenance.GetAivenMaintenanceUpdates[servicemaintenance.ValkeyMaintenanceUpdate](ctx, servicemaintenance.AivenDataLoaderKey{
		Project:     obj.AivenProject,
		ServiceName: obj.ServiceName,
	})
	if err != nil {
		if strings.Contains(err.Error(), "404 ServiceGet") {
			// Since obj is OK, we know the service exists within Kubernetes, but it might not yet be
			// fully created in Aiven. In that case, return a default maintenance window.
			return pagination.EmptyConnection[*servicemaintenance.ValkeyMaintenanceUpdate](), nil
		}
		return nil, err
	}

	return pagination.NewConnection(pagination.Slice(allUpdates, page), page, len(allUpdates)), nil
}

func (r *Resolver) OpenSearchMaintenance() gengql.OpenSearchMaintenanceResolver {
	return &openSearchMaintenanceResolver{r}
}

func (r *Resolver) ValkeyMaintenance() gengql.ValkeyMaintenanceResolver {
	return &valkeyMaintenanceResolver{r}
}

type (
	openSearchMaintenanceResolver struct{ *Resolver }
	valkeyMaintenanceResolver     struct{ *Resolver }
)
