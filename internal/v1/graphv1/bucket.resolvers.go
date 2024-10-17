package graphv1

import (
	"context"

	"github.com/nais/api/internal/v1/graphv1/gengqlv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/persistence/bucket"
	"github.com/nais/api/internal/v1/team"
	"github.com/nais/api/internal/v1/workload"
	"github.com/nais/api/internal/v1/workload/application"
	"github.com/nais/api/internal/v1/workload/job"
)

func (r *applicationResolver) Buckets(ctx context.Context, obj *application.Application, orderBy *bucket.BucketOrder) (*pagination.Connection[*bucket.Bucket], error) {
	if obj.Spec.GCP == nil {
		return pagination.EmptyConnection[*bucket.Bucket](), nil
	}

	return bucket.ListForWorkload(ctx, obj.TeamSlug, obj.Spec.GCP.Buckets, orderBy)
}

func (r *bucketResolver) Team(ctx context.Context, obj *bucket.Bucket) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *bucketResolver) Environment(ctx context.Context, obj *bucket.Bucket) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *bucketResolver) Workload(ctx context.Context, obj *bucket.Bucket) (workload.Workload, error) {
	return getWorkload(ctx, obj.WorkloadReference, obj.TeamSlug, obj.EnvironmentName)
}

func (r *jobResolver) Buckets(ctx context.Context, obj *job.Job, orderBy *bucket.BucketOrder) (*pagination.Connection[*bucket.Bucket], error) {
	if obj.Spec.GCP == nil {
		return pagination.EmptyConnection[*bucket.Bucket](), nil
	}
	return bucket.ListForWorkload(ctx, obj.TeamSlug, obj.Spec.GCP.Buckets, orderBy)
}

func (r *teamResolver) Buckets(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *bucket.BucketOrder) (*pagination.Connection[*bucket.Bucket], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return bucket.ListForTeam(ctx, obj.Slug, page, orderBy)
}

func (r *teamEnvironmentResolver) Bucket(ctx context.Context, obj *team.TeamEnvironment, name string) (*bucket.Bucket, error) {
	return bucket.Get(ctx, obj.TeamSlug, obj.Name, name)
}

func (r *teamInventoryCountsResolver) Buckets(ctx context.Context, obj *team.TeamInventoryCounts) (*bucket.TeamInventoryCountBuckets, error) {
	return &bucket.TeamInventoryCountBuckets{
		Total: len(bucket.ListAllForTeam(ctx, obj.TeamSlug)),
	}, nil
}

func (r *Resolver) Bucket() gengqlv1.BucketResolver { return &bucketResolver{r} }

type bucketResolver struct{ *Resolver }
