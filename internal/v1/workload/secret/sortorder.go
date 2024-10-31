package secret

import (
	"context"
	"strings"

	"github.com/nais/api/internal/v1/graphv1/sortfilter"
)

var SortFilter = sortfilter.New[*Secret, SecretOrderField, struct{}](SecretOrderFieldName)

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
}
