package grpcdatabase

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/grpc/grpcpagination"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/kubernetes/watchers"
	"github.com/nais/api/internal/persistence/postgres"
	"github.com/nais/api/internal/persistence/sqlinstance"
	"github.com/nais/api/pkg/apiclient/protoapi"
)

type Server struct {
	sqlDatabaseWatcher     *watchers.SqlDatabaseWatcher
	zalandoPostgresWatcher *watchers.ZalandoPostgresWatcher
	protoapi.UnimplementedDatabasesServer
}

func NewServer(sqlDatabaseWatcher *watchers.SqlDatabaseWatcher, zalandoPostgresWatcher *watchers.ZalandoPostgresWatcher) *Server {
	return &Server{
		sqlDatabaseWatcher:     sqlDatabaseWatcher,
		zalandoPostgresWatcher: zalandoPostgresWatcher,
	}
}

func (s *Server) List(_ context.Context, r *protoapi.ListDatabasesRequest) (*protoapi.ListDatabasesResponse, error) {
	sqlDatabases := watcher.Objects(s.sqlDatabaseWatcher.GetByNamespace(r.TeamSlug))
	postgresInstances := watcher.Objects(s.zalandoPostgresWatcher.GetByNamespace(r.TeamSlug))

	all := make([]*protoapi.Database, 0, len(sqlDatabases)+len(postgresInstances))
	for _, d := range sqlDatabases {
		all = append(all, sqlDatabaseToProto(d))
	}
	for _, p := range postgresInstances {
		all = append(all, postgresInstanceToProto(p))
	}

	// Sort by (type, environment, name, database) for deterministic pagination.
	slices.SortFunc(all, func(a, b *protoapi.Database) int {
		if c := strings.Compare(a.Type.String(), b.Type.String()); c != 0 {
			return c
		}
		if c := strings.Compare(a.Environment, b.Environment); c != 0 {
			return c
		}
		if c := strings.Compare(a.Name, b.Name); c != 0 {
			return c
		}
		return strings.Compare(a.Database, b.Database)
	})

	total := len(all)
	limit, offset := grpcpagination.Pagination(r)

	// Clamp to [0, total] with end >= start; limit/offset come from untrusted
	// clients and may be negative.
	start := min(max(int(offset), 0), total)
	end := min(max(start+int(max(limit, 0)), start), total)
	page := all[start:end]

	return &protoapi.ListDatabasesResponse{
		PageInfo: grpcpagination.PageInfo(r, total),
		Nodes:    page,
	}, nil
}

func sqlDatabaseToProto(d *sqlinstance.SQLDatabase) *protoapi.Database {
	return &protoapi.Database{
		Name:        d.SQLInstanceName,
		Database:    d.Name,
		Environment: d.EnvironmentName,
		TeamSlug:    d.TeamSlug.String(),
		Type:        protoapi.DatabaseType_CLOUD_SQL,
	}
}

func postgresInstanceToProto(p *postgres.PostgresInstance) *protoapi.Database {
	return &protoapi.Database{
		Name:        p.Name,
		Database:    "app",
		Environment: p.EnvironmentName,
		TeamSlug:    p.TeamSlug.String(),
		Type:        protoapi.DatabaseType_ZALANDO_POSTGRES,
	}
}
