package sqlinstance

import (
	"context"
	"strings"

	"github.com/nais/api/internal/v1/graphv1/sortfilter"
)

var (
	SortFilterSQLInstance     = sortfilter.New[*SQLInstance, SQLInstanceOrderField, struct{}](SQLInstanceOrderFieldName)
	SortFilterSQLInstanceUser = sortfilter.New[*SQLInstanceUser, SQLInstanceUserOrderField, struct{}](SQLInstanceUserOrderFieldName)
)

func init() {
	SortFilterSQLInstance.RegisterOrderBy(SQLInstanceOrderFieldName, func(ctx context.Context, a, b *SQLInstance) int {
		return strings.Compare(a.GetName(), b.GetName())
	})
	SortFilterSQLInstance.RegisterOrderBy(SQLInstanceOrderFieldEnvironment, func(ctx context.Context, a, b *SQLInstance) int {
		return strings.Compare(a.EnvironmentName, b.EnvironmentName)
	})
	SortFilterSQLInstance.RegisterOrderBy(SQLInstanceOrderFieldVersion, func(ctx context.Context, a, b *SQLInstance) int {
		if a.Version == nil && b.Version == nil {
			return 0
		} else if a.Version == nil {
			return 1
		} else if b.Version == nil {
			return -1
		}
		return strings.Compare(*a.Version, *b.Version)
	})

	SortFilterSQLInstanceUser.RegisterOrderBy(SQLInstanceUserOrderFieldName, func(ctx context.Context, a, b *SQLInstanceUser) int {
		return strings.Compare(a.Name, b.Name)
	})
	SortFilterSQLInstanceUser.RegisterOrderBy(SQLInstanceUserOrderFieldAuthentication, func(ctx context.Context, a, b *SQLInstanceUser) int {
		return strings.Compare(a.Authentication, b.Authentication)
	})
}
