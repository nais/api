package valkey

import (
	"context"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/slug"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
)

func GetByIdent(ctx context.Context, id ident.Ident) (*Valkey, error) {
	teamSlug, environment, name, err := parseIdent(id)
	if err != nil {
		return nil, err
	}

	return Get(ctx, teamSlug, environment, name)
}

func Get(ctx context.Context, teamSlug slug.Slug, environment, name string) (*Valkey, error) {
	return fromContext(ctx).client.watcher.Get(environment, teamSlug.String(), name)
}

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination, orderBy *ValkeyOrder) (*ValkeyConnection, error) {
	all := ListAllForTeam(ctx, teamSlug)
	orderValkey(ctx, all, orderBy)

	instances := pagination.Slice(all, page)
	return pagination.NewConnection(instances, page, len(all)), nil
}

func ListAllForTeam(ctx context.Context, teamSlug slug.Slug) []*Valkey {
	all := fromContext(ctx).client.watcher.GetByNamespace(teamSlug.String())
	return watcher.Objects(all)
}

func ListAccess(ctx context.Context, valkey *Valkey, page *pagination.Pagination, orderBy *ValkeyAccessOrder) (*ValkeyAccessConnection, error) {
	k8sClient := fromContext(ctx).client

	applicationAccess, err := k8sClient.getAccessForApplications(ctx, valkey.EnvironmentName, valkey.Name, valkey.TeamSlug)
	if err != nil {
		return nil, err
	}

	jobAccess, err := k8sClient.getAccessForJobs(ctx, valkey.EnvironmentName, valkey.Name, valkey.TeamSlug)
	if err != nil {
		return nil, err
	}

	all := make([]*ValkeyAccess, 0)
	all = append(all, applicationAccess...)
	all = append(all, jobAccess...)

	if orderBy == nil {
		orderBy = &ValkeyAccessOrder{
			Field:     "ACCESS",
			Direction: model.OrderDirectionAsc,
		}
	}
	SortFilterValkeyAccess.Sort(ctx, all, orderBy.Field, orderBy.Direction)

	ret := pagination.Slice(all, page)
	return pagination.NewConnection(ret, page, len(all)), nil
}

func ListForWorkload(ctx context.Context, teamSlug slug.Slug, environmentName string, references []nais_io_v1.Valkey, orderBy *ValkeyOrder) (*ValkeyConnection, error) {
	all := fromContext(ctx).client.watcher.GetByNamespace(teamSlug.String(), watcher.InCluster(environmentName))
	ret := make([]*Valkey, 0)

	for _, ref := range references {
		for _, d := range all {
			if d.Obj.Name == valkeyNamer(teamSlug, ref.Instance) {
				ret = append(ret, d.Obj)
			}
		}
	}

	orderValkey(ctx, ret, orderBy)
	return pagination.NewConnectionWithoutPagination(ret), nil
}

func orderValkey(ctx context.Context, instances []*Valkey, orderBy *ValkeyOrder) {
	if orderBy == nil {
		orderBy = &ValkeyOrder{
			Field:     "NAME",
			Direction: model.OrderDirectionAsc,
		}
	}

	SortFilterValkey.Sort(ctx, instances, orderBy.Field, orderBy.Direction)
}
