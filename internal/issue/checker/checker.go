package checker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/environmentmapper"
	"github.com/nais/api/internal/issue"
	"github.com/nais/api/internal/issue/checker/checkersql"
	"github.com/nais/api/internal/kubernetes/watchers"
	"github.com/nais/api/internal/leaderelection"
	"github.com/nais/api/internal/persistence/sqlinstance"
	"github.com/nais/api/internal/thirdparty/aiven"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const checkInterval = 1 * time.Minute

type check interface {
	Run(ctx context.Context) ([]Issue, error)
}

type Checker struct {
	checks        []check
	db            checkersql.Querier
	log           logrus.FieldLogger
	durationGauge metric.Float64Gauge
	issuesGauge   metric.Int64Gauge
}

type Config struct {
	AivenClient    aiven.AivenClient
	CloudSQLClient *sqlinstance.Client
	V13sClient     V13sClient
	Tenant         string
	Clusters       []string
}

type Issue struct {
	IssueType    issue.IssueType
	ResourceName string
	ResourceType issue.ResourceType
	Team         string
	Env          string
	Severity     issue.Severity
	Message      string
	IssueDetails any
}

func New(config Config, pool *pgxpool.Pool, watchers *watchers.Watchers, fakeEnabled bool, log logrus.FieldLogger) (*Checker, error) {
	meter := otel.GetMeterProvider().Meter("nais_api_issues")
	d, err := meter.Float64Gauge("nais_api_issue_checker_duration")
	if err != nil {
		return nil, fmt.Errorf("create duration gauge: %w", err)
	}
	i, err := meter.Int64Gauge("nais_api_issue_count")
	if err != nil {
		return nil, fmt.Errorf("create issue count gauge: %w", err)
	}
	checker := &Checker{
		db:            checkersql.New(pool),
		log:           log,
		durationGauge: d,
		issuesGauge:   i,
	}
	envs := Map(config.Clusters, func(c string) string { return environmentmapper.EnvironmentName(c) })

	v13s := config.V13sClient
	if fakeEnabled {
		v13s = &fakeV13sClient{}
	}

	checker.checks = []check{
		Aiven{aivenClient: config.AivenClient, tenant: config.Tenant, environments: envs, log: log.WithField("check", "Aiven")},
		SQLInstance{Client: config.CloudSQLClient, SQLInstanceWatcher: watchers.SqlInstanceWatcher, Log: log.WithField("check", "SQLInstance")},
		Workload{AppWatcher: *watchers.AppWatcher, JobWatcher: *watchers.JobWatcher, PodWatcher: *watchers.PodWatcher, RunWatcher: *watchers.RunWatcher, V13sClient: v13s, log: log.WithField("check", "Workload")},
	}

	return checker, nil
}

func (c *Checker) RunChecks(ctx context.Context) error {
	for {
		func() {
			if !leaderelection.IsLeader() {
				c.log.Debug("not leader, skipping checks")
				return
			}
			c.runChecks(ctx)
		}()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(checkInterval):
		}
	}
}

func (c *Checker) RunChecksOnce(ctx context.Context) {
	c.runChecks(ctx)
}

func (c *Checker) runChecks(ctx context.Context) {
	c.log.Debugf("starting checker")

	totalTime := time.Now()
	var issues []Issue
	for _, ch := range c.checks {
		checkTime := time.Now()
		checkIssues, err := ch.Run(ctx)
		if err != nil {
			c.log.WithError(err).Error("run check")
		}
		c.durationGauge.Record(ctx, time.Since(checkTime).Seconds(), metric.WithAttributes(attribute.String("operation", fmt.Sprintf("%T", ch))))
		issues = append(issues, checkIssues...)

	}
	c.durationGauge.Record(ctx, time.Since(totalTime).Seconds(), metric.WithAttributes(attribute.String("operation", "all_checks")))

	batchIssues := make([]checkersql.BatchInsertIssuesParams, 0)
	for _, i := range issues {
		d, err := json.Marshal(i.IssueDetails)
		if err != nil {
			c.log.WithError(err).Error("marshal issue details")
			continue
		}

		batchIssues = append(batchIssues, checkersql.BatchInsertIssuesParams{
			IssueType:    string(i.IssueType),
			ResourceName: i.ResourceName,
			ResourceType: string(i.ResourceType),
			Team:         i.Team,
			Env:          i.Env,
			Severity:     checkersql.SeverityLevel(i.Severity),
			Message:      i.Message,
			IssueDetails: d,
		})
	}

	dbTime := time.Now()

	err := c.db.DeleteIssues(ctx)
	if err != nil {
		c.log.WithError(err).Error("delete existing issues")
	}

	c.db.BatchInsertIssues(ctx, batchIssues).Exec(func(i int, err error) {
		if err != nil {
			c.log.WithField("index", i).WithError(err).Error("insert issue")
		}
	})
	c.durationGauge.Record(ctx, time.Since(dbTime).Seconds(), metric.WithAttributes(attribute.String("operation", "db")))
	c.recordIssues(ctx, issues)
	c.log.WithField("issues", len(issues)).Debug("issue checker finished")
}

func (c *Checker) recordIssues(ctx context.Context, issues []Issue) {
	type compoundKey struct {
		IssueType    string
		ResourceType string
		Severity     string
		Env          string
	}
	compoundCounter := make(map[compoundKey]int)
	for _, i := range issues {
		key := compoundKey{
			IssueType:    i.IssueType.String(),
			ResourceType: i.ResourceType.String(),
			Severity:     i.Severity.String(),
			Env:          i.Env,
		}
		compoundCounter[key]++
	}
	for key, count := range compoundCounter {
		c.issuesGauge.Record(
			ctx,
			int64(count),
			metric.WithAttributes(
				attribute.String("issue_type", key.IssueType),
				attribute.String("resource_type", key.ResourceType),
				attribute.String("severity", key.Severity),
				attribute.String("environment", key.Env),
			),
		)
	}
}

func Map[T any, U any](input []T, f func(T) U) []U {
	output := make([]U, len(input))
	for i, v := range input {
		output[i] = f(v)
	}
	return output
}
