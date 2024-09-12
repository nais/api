package bucket

import (
	"context"
	"slices"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/modelv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/kubernetes/watcher"
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
	outer := fromContext(ctx).watcher.GetByNamespace(teamSlug.String())

	all := watcher.Objects(outer)
	orderBuckets(all, orderBy)

	slice := pagination.Slice(all, page)
	return pagination.NewConnection(slice, page, int32(len(all))), nil
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

	orderBuckets(ret, orderBy)
	return pagination.NewConnectionWithoutPagination(ret), nil
}

func orderBuckets(buckets []*Bucket, orderBy *BucketOrder) {
	if orderBy == nil {
		orderBy = &BucketOrder{
			Field:     BucketOrderFieldName,
			Direction: modelv1.OrderDirectionAsc,
		}
	}
	switch orderBy.Field {
	case BucketOrderFieldName:
		slices.SortStableFunc(buckets, func(a, b *Bucket) int {
			return modelv1.Compare(a.Name, b.Name, orderBy.Direction)
		})
	}
}
