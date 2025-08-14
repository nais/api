package valkey

import (
	"context"
	"strings"

	"github.com/nais/api/internal/graph/sortfilter"
)

var (
	SortFilterValkey       = sortfilter.New[*Valkey, ValkeyOrderField, struct{}]()
	SortFilterValkeyAccess = sortfilter.New[*ValkeyAccess, ValkeyAccessOrderField, struct{}]()
)

func init() {
	SortFilterValkey.RegisterSort("NAME", func(ctx context.Context, a, b *Valkey) int {
		return strings.Compare(a.GetName(), b.GetName())
	}, "ENVIRONMENT")
	SortFilterValkey.RegisterSort("ENVIRONMENT", func(ctx context.Context, a, b *Valkey) int {
		return strings.Compare(a.EnvironmentName, b.EnvironmentName)
	}, "NAME")

	SortFilterValkeyAccess.RegisterSort("ACCESS", func(ctx context.Context, a, b *ValkeyAccess) int {
		return strings.Compare(a.Access, b.Access)
	})
	SortFilterValkeyAccess.RegisterSort("WORKLOAD", func(ctx context.Context, a, b *ValkeyAccess) int {
		return strings.Compare(a.WorkloadReference.Name, b.WorkloadReference.Name)
	})
}
