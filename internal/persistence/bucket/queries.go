package bucket

import (
	"context"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/search"
	"github.com/nais/api/internal/slug"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
)

func GetByIdent(ctx context.Context, id ident.Ident) (*Bucket, error) {
	teamSlug, environment, name, err := parseIdent(id)
	if err != nil {
		return nil, err
	}

	return Get(ctx, teamSlug, environment, name)
}

func Get(ctx context.Context, teamSlug slug.Slug, environment, name string) (*Bucket, error) {
	return fromContext(ctx).watcher.Get(environment, teamSlug.String(), name)
}

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination, orderBy *BucketOrder) (*BucketConnection, error) {
	all := ListAllForTeam(ctx, teamSlug)
	orderBuckets(ctx, all, orderBy)

	slice := pagination.Slice(all, page)
	return pagination.NewConnection(slice, page, int32(len(all))), nil
}

func ListAllForTeam(ctx context.Context, teamSlug slug.Slug) []*Bucket {
	all := fromContext(ctx).watcher.GetByNamespace(teamSlug.String())
	return watcher.Objects(all)
}

func ListForWorkload(ctx context.Context, teamSlug slug.Slug, references []nais_io_v1.CloudStorageBucket, orderBy *BucketOrder) (*BucketConnection, error) {
	all := fromContext(ctx).watcher.GetByNamespace(teamSlug.String())
	ret := make([]*Bucket, 0)

	for _, ref := range references {
		for _, d := range all {
			if d.Obj.Name == ref.Name {
				ret = append(ret, d.Obj)
			}
		}
	}

	orderBuckets(ctx, ret, orderBy)
	return pagination.NewConnectionWithoutPagination(ret), nil
}

func Search(ctx context.Context, q string) ([]*search.Result, error) {
	apps := fromContext(ctx).watcher.All()

	ret := make([]*search.Result, 0)
	for _, app := range apps {
		rank := search.Match(q, app.Obj.Name)
		if search.Include(rank) {
			ret = append(ret, &search.Result{
				Rank: rank,
				Node: app.Obj,
			})
		}
	}

	return ret, nil
}

func orderBuckets(ctx context.Context, buckets []*Bucket, orderBy *BucketOrder) {
	if orderBy == nil {
		orderBy = &BucketOrder{
			Field:     BucketOrderFieldName,
			Direction: model.OrderDirectionAsc,
		}
	}

	SortFilter.Sort(ctx, buckets, orderBy.Field, orderBy.Direction)
}
