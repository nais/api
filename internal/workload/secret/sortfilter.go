package secret

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/sortfilter"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

var SortFilter = sortfilter.New[*Secret, SecretOrderField, *SecretFilter](SecretOrderFieldName)

func init() {
	SortFilter.RegisterOrderBy(SecretOrderFieldName, func(ctx context.Context, a, b *Secret) int {
		return strings.Compare(a.GetName(), b.GetName())
	})
	SortFilter.RegisterOrderBy(SecretOrderFieldEnvironment, func(ctx context.Context, a, b *Secret) int {
		return strings.Compare(a.EnvironmentName, b.EnvironmentName)
	})
	SortFilter.RegisterOrderBy(SecretOrderFieldLastModifiedAt, func(ctx context.Context, a, b *Secret) int {
		if a.LastModifiedAt == nil && b.LastModifiedAt == nil {
			return 0
		}
		if a.LastModifiedAt == nil {
			return -1
		}
		if b.LastModifiedAt == nil {
			return 1
		}
		return a.LastModifiedAt.Compare(*b.LastModifiedAt)
	})
	SortFilter.RegisterFilter(func(ctx context.Context, v *Secret, filter *SecretFilter) bool {
		if filter.InUse == nil {
			return true
		}
		uses := 0

		applications := application.ListAllForTeam(ctx, v.TeamSlug)
		for _, app := range applications {
			if slices.Contains(app.GetSecrets(), v.Name) {
				uses++
			}
		}

		jobs := job.ListAllForTeam(ctx, v.TeamSlug)
		for _, j := range jobs {
			if slices.Contains(j.GetSecrets(), v.Name) {
				uses++
			}
		}

		return (uses > 0) == *filter.InUse
	})
}
