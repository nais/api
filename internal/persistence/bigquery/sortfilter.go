package bigquery

import (
	"context"
	"strings"

	"github.com/nais/api/internal/graph/sortfilter"
)

var (
	SortFilter       = sortfilter.New[*BigQueryDataset, BigQueryDatasetOrderField, struct{}]()
	SortFilterAccess = sortfilter.New[*BigQueryDatasetAccess, BigQueryDatasetAccessOrderField, struct{}]()
)

func init() {
	SortFilter.RegisterSort("NAME", func(ctx context.Context, a, b *BigQueryDataset) int {
		return strings.Compare(a.Name, b.Name)
	}, "ENVIRONMENT", "_OWNER", "_K8S_RESOURCE_NAME")

	SortFilter.RegisterSort("ENVIRONMENT", func(ctx context.Context, a, b *BigQueryDataset) int {
		return strings.Compare(a.EnvironmentName, b.EnvironmentName)
	}, "NAME", "_OWNER", "_K8S_RESOURCE_NAME")

	SortFilter.RegisterSort("_OWNER", func(ctx context.Context, a, b *BigQueryDataset) int {
		ownerA := a.TeamSlug.String()
		ownerB := b.TeamSlug.String()

		if a.WorkloadReference != nil {
			ownerA = a.WorkloadReference.Name
		}

		if b.WorkloadReference != nil {
			ownerB = b.WorkloadReference.Name
		}

		return strings.Compare(ownerA, ownerB)
	})

	SortFilter.RegisterSort("_K8S_RESOURCE_NAME", func(ctx context.Context, a, b *BigQueryDataset) int {
		return strings.Compare(a.K8sResourceName, b.K8sResourceName)
	})

	SortFilterAccess.RegisterSort("EMAIL", func(ctx context.Context, a, b *BigQueryDatasetAccess) int {
		return strings.Compare(a.Email, b.Email)
	})

	SortFilterAccess.RegisterSort("ROLE", func(ctx context.Context, a, b *BigQueryDatasetAccess) int {
		return strings.Compare(a.Role, b.Role)
	})
}
