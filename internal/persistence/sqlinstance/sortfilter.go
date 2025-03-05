package sqlinstance

import (
	"context"
	"strings"

	"github.com/nais/api/internal/cost"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/sortfilter"
	"k8s.io/utils/ptr"
)

var (
	SortFilterSQLInstance     = sortfilter.New[*SQLInstance, SQLInstanceOrderField, struct{}]()
	SortFilterSQLInstanceUser = sortfilter.New[*SQLInstanceUser, SQLInstanceUserOrderField, struct{}]()
)

type SortFilterTieBreaker = sortfilter.TieBreaker[SQLInstanceOrderField]

func init() {
	SortFilterSQLInstance.RegisterSort("NAME", func(ctx context.Context, a, b *SQLInstance) int {
		return strings.Compare(a.GetName(), b.GetName())
	}, SortFilterTieBreaker{
		Field:     "ENVIRONMENT",
		Direction: ptr.To(model.OrderDirectionAsc),
	})
	SortFilterSQLInstance.RegisterSort("ENVIRONMENT", func(ctx context.Context, a, b *SQLInstance) int {
		return strings.Compare(a.EnvironmentName, b.EnvironmentName)
	}, SortFilterTieBreaker{
		Field:     "NAME",
		Direction: ptr.To(model.OrderDirectionAsc),
	})
	SortFilterSQLInstance.RegisterSort("VERSION", func(ctx context.Context, a, b *SQLInstance) int {
		if a.Version == nil && b.Version == nil {
			return 0
		} else if a.Version == nil {
			return 1
		} else if b.Version == nil {
			return -1
		}
		return strings.Compare(*a.Version, *b.Version)
	}, SortFilterTieBreaker{
		Field:     "NAME",
		Direction: ptr.To(model.OrderDirectionAsc),
	}, SortFilterTieBreaker{
		Field:     "ENVIRONMENT",
		Direction: ptr.To(model.OrderDirectionAsc),
	})
	SortFilterSQLInstance.RegisterConcurrentSort("STATUS", func(ctx context.Context, a *SQLInstance) int {
		stateOrder := map[string]int{
			"UNSPECIFIED":    0,
			"RUNNABLE":       1,
			"SUSPENDED":      2,
			"PENDING_DELETE": 3,
			"PENDING_CREATE": 4,
			"MAINTENANCE":    5,
			"FAILED":         6,
		}

		aState, err := GetState(ctx, a.ProjectID, a.Name)
		if err != nil {
			return 0
		}

		return stateOrder[aState.String()]
	})
	SortFilterSQLInstance.RegisterConcurrentSort("COST", func(ctx context.Context, a *SQLInstance) int {
		if a.WorkloadReference == nil {
			return 0
		}
		aCost, err := cost.MonthlyForService(ctx, a.TeamSlug, a.EnvironmentName, a.WorkloadReference.Name, "Cloud SQL")
		if err != nil {
			return 0
		}

		return int(aCost * 100)
	})
	SortFilterSQLInstance.RegisterConcurrentSort("CPU_UTILIZATION", func(ctx context.Context, a *SQLInstance) int {
		aCPU, err := CPUForInstance(ctx, a.ProjectID, a.Name)
		if err != nil {
			return 0
		}

		if aCPU == nil {
			return 0
		}

		return int(aCPU.Utilization * 100)
	})
	SortFilterSQLInstance.RegisterConcurrentSort("MEMORY_UTILIZATION", func(ctx context.Context, a *SQLInstance) int {
		aMemory, err := MemoryForInstance(ctx, a.ProjectID, a.Name)
		if err != nil {
			return 0
		}

		if aMemory == nil {
			return 0
		}

		return int(aMemory.Utilization * 100)
	})
	SortFilterSQLInstance.RegisterConcurrentSort("DISK_UTILIZATION", func(ctx context.Context, a *SQLInstance) int {
		aDisk, err := DiskForInstance(ctx, a.ProjectID, a.Name)
		if err != nil {
			return 0
		}

		if aDisk == nil {
			return 0
		}

		return int(aDisk.Utilization * 100)
	})

	// SQLInstanceUser
	SortFilterSQLInstanceUser.RegisterSort("NAME", func(ctx context.Context, a, b *SQLInstanceUser) int {
		return strings.Compare(a.Name, b.Name)
	})
	SortFilterSQLInstanceUser.RegisterSort("AUTHENTICATION", func(ctx context.Context, a, b *SQLInstanceUser) int {
		return strings.Compare(a.Authentication, b.Authentication)
	})
}
