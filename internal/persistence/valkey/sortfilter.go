package valkey

import (
	"context"
	"strings"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/sortfilter"
)

var (
	SortFilterValkeyInstance       = sortfilter.New[*ValkeyInstance, ValkeyInstanceOrderField, struct{}]("NAME", model.OrderDirectionAsc)
	SortFilterValkeyInstanceAccess = sortfilter.New[*ValkeyInstanceAccess, ValkeyInstanceAccessOrderField, struct{}]("ACCESS", model.OrderDirectionAsc)
)

func init() {
	SortFilterValkeyInstance.RegisterOrderBy("NAME", func(ctx context.Context, a, b *ValkeyInstance) int {
		return strings.Compare(a.GetName(), b.GetName())
	})
	SortFilterValkeyInstance.RegisterOrderBy("ENVIRONMENT", func(ctx context.Context, a, b *ValkeyInstance) int {
		return strings.Compare(a.EnvironmentName, b.EnvironmentName)
	})

	SortFilterValkeyInstanceAccess.RegisterOrderBy("ACCESS", func(ctx context.Context, a, b *ValkeyInstanceAccess) int {
		return strings.Compare(a.Access, b.Access)
	})
	SortFilterValkeyInstanceAccess.RegisterOrderBy("WORKLOAD", func(ctx context.Context, a, b *ValkeyInstanceAccess) int {
		return strings.Compare(a.WorkloadReference.Name, b.WorkloadReference.Name)
	})
}
