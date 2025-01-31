package database

import (
	"context"
	"embed"
	"fmt"
	"regexp"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
	"github.com/sirupsen/logrus"

	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed migrations/0*.sql
var embedMigrations embed.FS

const databaseConnectRetries = 5

var regParseSQLName = regexp.MustCompile(`\-\-\s*name:\s+(\S+)`)

type settings struct {
	slowQueryLogger time.Duration
}

type OptFunc func(*settings)

// WithSlowQueryLogger enables slow query logging
// This exposes attributes of the slow query to the logger, so it should not be used in production
func WithSlowQueryLogger(d time.Duration) OptFunc {
	return func(s *settings) {
		s.slowQueryLogger = d
	}
}

func New(ctx context.Context, dsn string, log logrus.FieldLogger, opts ...OptFunc) (*pgxpool.Pool, error) {
	conn, err := NewPool(ctx, dsn, log, true, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to the database: %w", err)
	}
	return conn, nil
}

func NewPool(ctx context.Context, dsn string, log logrus.FieldLogger, migrate bool, opts ...OptFunc) (*pgxpool.Pool, error) {
	settings := &settings{}
	for _, o := range opts {
		o(settings)
	}

	if migrate {
		if err := migrateDatabaseSchema("pgx", dsn, log); err != nil {
			return nil, err
		}
	}

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse dsn config: %w", err)
	}
	config.MaxConns = 25
	config.ConnConfig.Tracer = otelpgx.NewTracer(
		otelpgx.WithTrimSQLInSpanName(),
		otelpgx.WithSpanNameFunc(func(stmt string) string {
			matches := regParseSQLName.FindStringSubmatch(stmt)
			if len(matches) > 1 {
				return matches[1]
			}

			return "unknown"
		}),
	)

	if settings.slowQueryLogger > 0 {
		config.ConnConfig.Tracer = &slowQueryLogger{
			log:      log,
			sub:      config.ConnConfig.Tracer,
			duration: settings.slowQueryLogger,
		}
	}

	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		t, err := conn.LoadType(ctx, "slug") // slug type
		if err != nil {
			return fmt.Errorf("failed to load type: %w", err)
		}
		conn.TypeMap().RegisterType(t)

		t, err = conn.LoadType(ctx, "_slug") // array of slug type
		if err != nil {
			return fmt.Errorf("failed to load type: %w", err)
		}
		conn.TypeMap().RegisterType(t)
		return nil
	}

	conn, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	connected := false
	for i := 0; i < databaseConnectRetries; i++ {
		if err = conn.Ping(ctx); err == nil {
			connected = true
			break
		}

		time.Sleep(time.Second * time.Duration(i+1))
	}

	if !connected {
		return nil, fmt.Errorf("giving up connecting to the database after %d attempts: %w", databaseConnectRetries, err)
	}

	return conn, nil
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

type slowQueryLogger struct {
	log      logrus.FieldLogger
	sub      pgx.QueryTracer
	duration time.Duration
}

type sqlCtx int

const sqlCtxKey sqlCtx = 0

type bucket struct {
	pgx.TraceQueryStartData
	start time.Time
}

func (s *slowQueryLogger) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	ctx = s.sub.TraceQueryStart(ctx, conn, data)
	return context.WithValue(ctx, sqlCtxKey, &bucket{
		TraceQueryStartData: data,
		start:               time.Now(),
	})
}

func (s *slowQueryLogger) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	s.sub.TraceQueryEnd(ctx, conn, data)

	b, ok := ctx.Value(sqlCtxKey).(*bucket)
	if !ok {
		return
	}

	dur := time.Since(b.start)
	if dur > s.duration {
		s.log.WithFields(logrus.Fields{
			"sql":  b.SQL,
			"args": b.Args,
			"time": dur,
			"err":  data.Err,
		}).Warn("slow query")
	}
}
