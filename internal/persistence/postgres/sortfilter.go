package postgres

import (
	"context"
	"strings"

	"github.com/nais/api/internal/graph/sortfilter"
)

var SortFilterPostgresInstance = sortfilter.New[*PostgresInstance, PostgresInstanceOrderField, struct{}]()

func init() {
	SortFilterPostgresInstance.RegisterSort("NAME", func(ctx context.Context, a, b *PostgresInstance) int {
		return strings.Compare(a.GetName(), b.GetName())
	}, "ENVIRONMENT")
	SortFilterPostgresInstance.RegisterSort("ENVIRONMENT", func(ctx context.Context, a, b *PostgresInstance) int {
		return strings.Compare(a.EnvironmentName, b.EnvironmentName)
	}, "NAME")
}
