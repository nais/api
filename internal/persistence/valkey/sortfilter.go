package valkey

import (
	"context"
	"strings"

	"github.com/nais/api/internal/graph/sortfilter"
)

var (
	SortFilterValkeyInstance       = sortfilter.New[*ValkeyInstance, ValkeyInstanceOrderField, struct{}]()
	SortFilterValkeyInstanceAccess = sortfilter.New[*ValkeyInstanceAccess, ValkeyInstanceAccessOrderField, struct{}]()
)

func init() {
	SortFilterValkeyInstance.RegisterSort("NAME", func(ctx context.Context, a, b *ValkeyInstance) int {
		return strings.Compare(a.GetName(), b.GetName())
	}, "ENVIRONMENT")
	SortFilterValkeyInstance.RegisterSort("ENVIRONMENT", func(ctx context.Context, a, b *ValkeyInstance) int {
		return strings.Compare(a.EnvironmentName, b.EnvironmentName)
	}, "NAME")

	SortFilterValkeyInstanceAccess.RegisterSort("ACCESS", func(ctx context.Context, a, b *ValkeyInstanceAccess) int {
		return strings.Compare(a.Access, b.Access)
	})
	SortFilterValkeyInstanceAccess.RegisterSort("WORKLOAD", func(ctx context.Context, a, b *ValkeyInstanceAccess) int {
		return strings.Compare(a.WorkloadReference.Name, b.WorkloadReference.Name)
	})
}
