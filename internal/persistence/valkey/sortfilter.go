package valkey

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/sortfilter"
)

var (
	SortFilterValkey       = sortfilter.New[*Valkey, ValkeyOrderField, *ValkeyFilter]()
	SortFilterValkeyAccess = sortfilter.New[*ValkeyAccess, ValkeyAccessOrderField, struct{}]()
)

func init() {
	SortFilterValkey.RegisterSort("NAME", func(ctx context.Context, a, b *Valkey) int {
		return strings.Compare(a.GetName(), b.GetName())
	}, "ENVIRONMENT")
	SortFilterValkey.RegisterSort("ENVIRONMENT", func(ctx context.Context, a, b *Valkey) int {
		return strings.Compare(a.EnvironmentName, b.EnvironmentName)
	}, "NAME")

	SortFilterValkey.RegisterConcurrentSort("STATE", func(ctx context.Context, a *Valkey) int {
		s, err := State(ctx, a)
		if err != nil {
			return int(ValkeyStateUnknown)
		}

		return int(s)
	}, "NAME")

	SortFilterValkey.RegisterFilter(func(ctx context.Context, v *Valkey, filter *ValkeyFilter) bool {
		if filter.Name != "" {
			if !strings.Contains(strings.ToLower(v.Name), strings.ToLower(filter.Name)) {
				return false
			}
		}

		if len(filter.Environments) > 0 {
			if !slices.Contains(filter.Environments, v.EnvironmentName) {
				return false
			}
		}

		if len(filter.Tiers) > 0 {
			if !slices.Contains(filter.Tiers, v.Tier) {
				return false
			}
		}

		return true
	})

	SortFilterValkeyAccess.RegisterSort("ACCESS", func(ctx context.Context, a, b *ValkeyAccess) int {
		return strings.Compare(a.Access, b.Access)
	})
	SortFilterValkeyAccess.RegisterSort("WORKLOAD", func(ctx context.Context, a, b *ValkeyAccess) int {
		return strings.Compare(a.WorkloadReference.Name, b.WorkloadReference.Name)
	})
}
