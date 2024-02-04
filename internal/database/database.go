package database

import (
	"context"
	"embed"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/nais/api/internal/database/gensql"
	"github.com/pressly/goose/v3"
	"github.com/sirupsen/logrus"
)

//go:embed migrations/0*.sql
var embedMigrations embed.FS

const databaseConnectRetries = 5

type (
	QuerierTransactionFunc  func(ctx context.Context, querier Querier) error
	DatabaseTransactionFunc func(ctx context.Context, dbtx Database) error
)

type Database interface {
	AuditLogsRepo
	CostRepo
	FirstRunRepo
	ReconcilerErrorRepo
	ReconcilerRepo
	ReconcilerStateRepo
	RepositoryAuthorizationRepo
	ResourceUtilizationRepo
	RoleRepo
	ServiceAccountRepo
	SessionRepo
	TeamRepo
	UserRepo
	VulnerabilityMetricsRepo

	Transaction(ctx context.Context, fn DatabaseTransactionFunc) error
}

type Querier interface {
	gensql.Querier
	Transaction(ctx context.Context, callback QuerierTransactionFunc) error
}

type Queries struct {
	*gensql.Queries
	connPool *pgxpool.Pool
}

func (q *Queries) Transaction(ctx context.Context, callback QuerierTransactionFunc) error {
	tx, err := q.connPool.Begin(ctx)
	if err != nil {
		return err
	}

	defer tx.Rollback(ctx)

	qtx := &Queries{
		Queries:  q.Queries.WithTx(tx),
		connPool: q.connPool,
	}

	if err := callback(ctx, qtx); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

type database struct {
	querier Querier
}

func (d *database) Transaction(ctx context.Context, fn DatabaseTransactionFunc) error {
	return d.querier.Transaction(ctx, func(ctx context.Context, querier Querier) error {
		return fn(ctx, &database{querier: querier})
	})
}

// New connects to the database, runs migrations and returns a database instance. The caller must call the
// returned closer function when the database connection is no longer needed
func New(ctx context.Context, dsn string, log logrus.FieldLogger) (db Database, closer func(), err error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse dsn config: %w", err)
	}

	conn, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect: %w", err)
	}

	connected := false
	for i := 0; i < databaseConnectRetries; i++ {
		err = conn.Ping(ctx)
		if err == nil {
			connected = true
			break
		}

		time.Sleep(time.Second * time.Duration(i+1))
	}

	if !connected {
		return nil, nil, fmt.Errorf("giving up connecting to the database after %d attempts: %w", databaseConnectRetries, err)
	}

	if err = migrateDatabaseSchema("pgx", dsn, log); err != nil {
		return nil, nil, err
	}

	return &database{
		querier: &Queries{
			Queries:  gensql.New(conn),
			connPool: conn,
		},
	}, conn.Close, nil
}

// migrateDatabaseSchema runs database migrations
func migrateDatabaseSchema(driver, dsn string, log logrus.FieldLogger) error {
	goose.SetBaseFS(embedMigrations)
	goose.SetLogger(log)

	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	db, err := goose.OpenDBWithDriver(driver, dsn)
	if err != nil {
		return err
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.WithError(err).Error("closing database migration connection")
		}
	}()

	return goose.Up(db, "migrations")
}
