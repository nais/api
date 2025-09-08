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
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/kubernetes/watchers"
	"github.com/nais/api/internal/leaderelection"
	"github.com/nais/api/internal/persistence/sqlinstance"
	"github.com/nais/api/internal/thirdparty/aiven"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const checkInterval = 5 * time.Minute

type check interface {
	Run(ctx context.Context) ([]Issue, error)
}

type KubernetesLister[T any] interface {
	List(ctx context.Context) []T
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

type jobLister struct {
	watcher *watchers.JobWatcher
}
type applicationLister struct {
	watcher *watchers.AppWatcher
}

type options struct {
	sqlInstanceLister KubernetesLister[*sqlinstance.SQLInstance]
	applicationLister KubernetesLister[*watcher.EnvironmentWrapper[*nais_io_v1alpha1.Application]]
	jobLister         KubernetesLister[*watcher.EnvironmentWrapper[*nais_io_v1.Naisjob]]
}
type Option func(*options)

func New(config Config, pool *pgxpool.Pool, watchers *watchers.Watchers, log logrus.FieldLogger, opts ...Option) (*Checker, error) {
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
	o := &options{
		sqlInstanceLister: &sqlInstanceLister{watcher: watchers.SqlInstanceWatcher},
		applicationLister: &applicationLister{watcher: watchers.AppWatcher},
		jobLister:         &jobLister{watcher: watchers.JobWatcher},
	}

	for _, opt := range opts {
		opt(o)
	}

	checker.checks = []check{
		Aiven{aivenClient: config.AivenClient, tenant: config.Tenant, environments: envs, log: log.WithField("check", "Aiven")},
		SQLInstance{Client: config.CloudSQLClient, SQLInstanceLister: o.sqlInstanceLister, Log: log.WithField("check", "SQLInstance")},
		Workload{ApplicationLister: o.applicationLister, JobLister: o.jobLister},
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
		c.issuesGauge.Record(ctx, int64(len(checkIssues)), metric.WithAttributes(attribute.String("check", fmt.Sprintf("%T", ch))))
		issues = append(issues, checkIssues...)

	}
	c.issuesGauge.Record(ctx, int64(len(issues)), metric.WithAttributes(attribute.String("check", "all_checks")))
	c.durationGauge.Record(ctx, time.Since(totalTime).Seconds(), metric.WithAttributes(attribute.String("operation", "all_checks")))

	batchIssues := make([]checkersql.BatchInsertIssuesParams, 0)
	for _, issue := range issues {
		d, err := json.Marshal(issue.IssueDetails)
		if err != nil {
			c.log.WithError(err).Error("marshal issue details")
			continue
		}

		batchIssues = append(batchIssues, checkersql.BatchInsertIssuesParams{
			IssueType:    string(issue.IssueType),
			ResourceName: issue.ResourceName,
			ResourceType: string(issue.ResourceType),
			Team:         issue.Team,
			Env:          issue.Env,
			Severity:     string(issue.Severity),
			Message:      issue.Message,
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
	counter := make(map[issue.IssueType]int)
	for _, issue := range issues {
		counter[issue.IssueType]++
	}
	for issueType, count := range counter {
		c.issuesGauge.Record(ctx, int64(count), metric.WithAttributes(attribute.String("issue_type", issueType.String())))
	}
}

func WithSQLInstanceLister(lister KubernetesLister[*sqlinstance.SQLInstance]) Option {
	return func(o *options) {
		o.sqlInstanceLister = lister
	}
}

func WithApplicationLister(lister KubernetesLister[*watcher.EnvironmentWrapper[*nais_io_v1alpha1.Application]]) Option {
	return func(o *options) {
		o.applicationLister = lister
	}
}

func (a *applicationLister) List(_ context.Context) []*watcher.EnvironmentWrapper[*nais_io_v1alpha1.Application] {
	return a.watcher.All()
}

func (j *jobLister) List(_ context.Context) []*watcher.EnvironmentWrapper[*nais_io_v1.Naisjob] {
	return j.watcher.All()
}

func Map[T any, U any](input []T, f func(T) U) []U {
	output := make([]U, len(input))
	for i, v := range input {
		output[i] = f(v)
	}
	return output
}
