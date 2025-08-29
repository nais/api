package checker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/environment"
	"github.com/nais/api/internal/issue"
	"github.com/nais/api/internal/issue/checker/checkersql"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/kubernetes/watchers"
	"github.com/nais/api/internal/persistence/sqlinstance"
	"github.com/nais/api/internal/thirdparty/aivencache"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	"github.com/sirupsen/logrus"
)

const CheckInterval = 1 * time.Minute

type Check interface {
	Run(ctx context.Context) ([]Issue, error)
}

type KubernetesLister[T any] interface {
	List(ctx context.Context) []T
}

type Checker struct {
	Checks []Check
	Db     checkersql.Querier
	Log    logrus.FieldLogger
}

type Config struct {
	AivenClient    aivencache.AivenClient
	CloudSQLClient *sqlinstance.Client
	Tenant         string
}

type Issue struct {
	IssueType    issue.IssueType
	ResourceName string
	ResourceType issue.ResourceType
	Team         string
	Env          string
	Severity     issue.Severity
	IssueDetails any
}

type ApplicationLister struct {
	Environments []string
	watcher      *watchers.AppWatcher
}

type options struct {
	SQLInstanceLister KubernetesLister[*sqlinstance.SQLInstance]
	ApplicationLister KubernetesLister[*watcher.EnvironmentWrapper[*nais_io_v1alpha1.Application]]
}
type Option func(*options)

func New(ctx context.Context, config Config, pool *pgxpool.Pool, watchers *watchers.Watchers, log logrus.FieldLogger, opts ...Option) (*Checker, error) {
	ctx = environment.NewLoaderContext(ctx, pool)
	e, err := environment.List(ctx, nil)
	if err != nil {
		panic(fmt.Sprintf("failed to list environments: %v", err))
	}
	checker := &Checker{
		Db:  checkersql.New(pool),
		Log: log,
	}
	envs := Map(e, func(e *environment.Environment) string { return e.Name })
	o := &options{
		SQLInstanceLister: &SQLInstanceLister{watcher: watchers.SqlInstanceWatcher},
		ApplicationLister: &ApplicationLister{watcher: watchers.AppWatcher, Environments: envs},
	}

	for _, opt := range opts {
		opt(o)
	}

	checker.Checks = []Check{
		Aiven{AivenClient: config.AivenClient, Tenant: config.Tenant, Environments: envs, Log: log.WithField("check", "Aiven")},
		SQLInstance{Client: config.CloudSQLClient, SQLInstanceLister: o.SQLInstanceLister, Log: log.WithField("check", "SQLInstance")},
		DeprecatedIngress{ApplicationLister: o.ApplicationLister},
	}

	return checker, nil
}

func (c *Checker) RunChecks(ctx context.Context) error {
	// initial run
	c.runChecks(ctx)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(CheckInterval):
			c.runChecks(ctx)
		}
	}
}

func (c *Checker) RunChecksOnce(ctx context.Context) {
	c.runChecks(ctx)
}

func (c *Checker) runChecks(ctx context.Context) {
	c.Log.Debugf("starting checker")
	var issues []Issue
	for _, check := range c.Checks {
		checkIssues, err := check.Run(ctx)
		if err != nil {
			c.Log.WithError(err).Error("run check")
		}
		issues = append(issues, checkIssues...)
	}

	batchIssues := make([]checkersql.BatchInsertIssuesParams, 0)
	for _, issue := range issues {
		d, err := json.Marshal(issue.IssueDetails)
		if err != nil {
			c.Log.WithError(err).Error("marshal issue details")
			continue
		}

		batchIssues = append(batchIssues, checkersql.BatchInsertIssuesParams{
			IssueType:    string(issue.IssueType),
			ResourceName: issue.ResourceName,
			ResourceType: string(issue.ResourceType),
			Team:         issue.Team,
			Env:          issue.Env,
			Severity:     string(issue.Severity),
			IssueDetails: d,
		})
	}

	err := c.Db.DeleteIssues(ctx)
	if err != nil {
		c.Log.WithError(err).Error("delete existing issues")
	}

	c.Db.BatchInsertIssues(ctx, batchIssues).Exec(func(i int, err error) {
		if err != nil {
			c.Log.Errorf("insert issue %d: %v", i, err)
		}
	})
	c.Log.Debugf("checker finished, found %d issues", len(issues))
}

func WithSQLInstanceLister(lister KubernetesLister[*sqlinstance.SQLInstance]) Option {
	return func(o *options) {
		o.SQLInstanceLister = lister
	}
}

func WithApplicationLister(lister KubernetesLister[*watcher.EnvironmentWrapper[*nais_io_v1alpha1.Application]]) Option {
	return func(o *options) {
		o.ApplicationLister = lister
	}
}

func (a *ApplicationLister) List(_ context.Context) []*watcher.EnvironmentWrapper[*nais_io_v1alpha1.Application] {
	return a.watcher.All()
}

func Map[T any, U any](input []T, f func(T) U) []U {
	output := make([]U, len(input))
	for i, v := range input {
		output[i] = f(v)
	}
	return output
}
