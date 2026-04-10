package graph

import (
	"context"

	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/instancegroup"
)

func (r *applicationResolver) InstanceGroups(ctx context.Context, obj *application.Application) ([]*instancegroup.InstanceGroup, error) {
	return instancegroup.ListForApplication(ctx, obj.TeamSlug, obj.EnvironmentName, obj.Name)
}

func (r *instanceGroupResolver) EnvironmentVariables(ctx context.Context, obj *instancegroup.InstanceGroup) ([]*instancegroup.InstanceGroupEnvironmentVariable, error) {
	return instancegroup.ListEnvironmentVariables(ctx, obj)
}

func (r *instanceGroupResolver) MountedFiles(ctx context.Context, obj *instancegroup.InstanceGroup) ([]*instancegroup.InstanceGroupMountedFile, error) {
	return instancegroup.ListMountedFiles(ctx, obj)
}

func (r *instanceGroupResolver) Instances(ctx context.Context, obj *instancegroup.InstanceGroup) ([]*application.ApplicationInstance, error) {
	return instancegroup.ListInstances(ctx, obj)
}

func (r *instanceGroupResolver) Events(ctx context.Context, obj *instancegroup.InstanceGroup) ([]*instancegroup.InstanceGroupEvent, error) {
	return instancegroup.ListEvents(ctx, obj)
}

func (r *Resolver) InstanceGroup() gengql.InstanceGroupResolver { return &instanceGroupResolver{r} }

type instanceGroupResolver struct{ *Resolver }
