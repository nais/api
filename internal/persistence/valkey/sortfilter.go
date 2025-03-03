package valkey

import (
	"context"
	"strings"

	"github.com/nais/api/internal/graph/sortfilter"
)

var (
	SortFilterValkeyInstance       = sortfilter.New[*ValkeyInstance, ValkeyInstanceOrderField, struct{}](ValkeyInstanceOrderFieldName)
	SortFilterValkeyInstanceAccess = sortfilter.New[*ValkeyInstanceAccess, ValkeyInstanceAccessOrderField, struct{}](ValkeyInstanceAccessOrderFieldAccess)
)

func init() {
	SortFilterValkeyInstance.RegisterOrderBy(ValkeyInstanceOrderFieldName, func(ctx context.Context, a, b *ValkeyInstance) int {
		return strings.Compare(a.GetName(), b.GetName())
	})
	SortFilterValkeyInstance.RegisterOrderBy(ValkeyInstanceOrderFieldEnvironment, func(ctx context.Context, a, b *ValkeyInstance) int {
		return strings.Compare(a.EnvironmentName, b.EnvironmentName)
	})

	SortFilterValkeyInstanceAccess.RegisterOrderBy(ValkeyInstanceAccessOrderFieldAccess, func(ctx context.Context, a, b *ValkeyInstanceAccess) int {
		return strings.Compare(a.Access, b.Access)
	})
	SortFilterValkeyInstanceAccess.RegisterOrderBy(ValkeyInstanceAccessOrderFieldWorkload, func(ctx context.Context, a, b *ValkeyInstanceAccess) int {
		return strings.Compare(a.WorkloadReference.Name, b.WorkloadReference.Name)
	})
}
