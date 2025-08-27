package checker

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"

	aiven "github.com/aiven/go-client-codegen"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/environment"
	"github.com/nais/api/internal/issue/checker/checkersql"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/kubernetes/watchers"
	"github.com/nais/api/internal/persistence/sqlinstance"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	"google.golang.org/api/sqladmin/v1"
)

type Issue struct {
	IssueType    IssueType
	ResourceName string
	ResourceType string
	Team         string
	Env          string
	Severity     Severity
	IssueDetails any
}
type IssueType string

const (
	IssueTypeAivenIssue        IssueType = "AIVEN_ISSUE"
	IssueTypeSQLInstanceIssue  IssueType = "SQLINSTANCE_ISSUE"
	IssueTypeDeprecatedIngress IssueType = "DEPRECATED_INGRESS"
)

type Severity string

const (
	SeverityCritical Severity = "CRITICAL"
	SeverityWarning  Severity = "WARNING"
	SeverityTodo     Severity = "TODO"
)

type Check interface {
	Run(ctx context.Context) ([]Issue, error)
}

type Checker struct {
	Checks []Check
	Db     checkersql.Querier
}

type Option func(*options)

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

type KubernetesLister[T any] interface {
	List(ctx context.Context) []T
}

type ApplicationLister struct {
	Environments []string
	watcher      *watchers.AppWatcher
}

type options struct {
	SQLInstanceLister KubernetesLister[*sqlinstance.SQLInstance]
	ApplicationLister KubernetesLister[*watcher.EnvironmentWrapper[*nais_io_v1alpha1.Application]]
}

func New(ctx context.Context, config Config, pool *pgxpool.Pool, watchers *watchers.Watchers, opts ...Option) (*Checker, error) {
	ctx = environment.NewLoaderContext(ctx, pool)
	e, err := environment.List(ctx, nil)
	if err != nil {
		panic(fmt.Sprintf("failed to list environments: %v", err))
	}
	checker := &Checker{
		Db: checkersql.New(pool),
	}
	envs := Map(e, func(e *environment.Environment) string { return e.Name })
	options := &options{
		SQLInstanceLister: &SQLInstanceLister{watcher: watchers.SqlInstanceWatcher},
		ApplicationLister: &ApplicationLister{watcher: watchers.AppWatcher, Environments: envs},
	}

	for _, opt := range opts {
		opt(options)
	}

	a, err := aiven.NewClient(aiven.TokenOpt(config.AivenToken), aiven.UserAgentOpt("nais-api"))
	if err != nil {
		return nil, err
	}

	sqladmin, err := sqladmin.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create SQL Admin service: %w", err)
	}

	checker.Checks = []Check{
		Aiven{AivenClient: a, Tenant: config.Tenant, Environments: envs},
		SQLInstance{SQLInstanceClient: sqladmin.Instances, SQLInstanceLister: options.SQLInstanceLister},
		DeprecatedIngress{ApplicationLister: options.ApplicationLister},
	}

	return checker, nil
}

func (a *ApplicationLister) List(ctx context.Context) []*watcher.EnvironmentWrapper[*nais_io_v1alpha1.Application] {
	return a.watcher.All()
}

type Config struct {
	AivenToken string
	Tenant     string
}

func (c *Checker) RunChecks(ctx context.Context) error {
	var issues []Issue
	for _, check := range c.Checks {
		checkIssues, err := check.Run(ctx)
		if err != nil {
			logrus.WithError(err).Error("failed to run check")
		}
		issues = append(issues, checkIssues...)
	}

	batchIssues := make([]checkersql.BatchInsertIssuesParams, 0)
	for _, issue := range issues {
		println("Found issue:", issue.ResourceName, "of type", issue.IssueType)
		d, err := json.Marshal(issue.IssueDetails)
		if err != nil {
			return err
		}

		batchIssues = append(batchIssues, checkersql.BatchInsertIssuesParams{
			IssueType:    string(issue.IssueType),
			ResourceName: issue.ResourceName,
			ResourceType: issue.ResourceType,
			Team:         issue.Team,
			Env:          issue.Env,
			Severity:     string(issue.Severity),
			IssueDetails: d,
		})
	}
	err := c.Db.DeleteIssues(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete existing issues: %w", err)
	}

	// TODO: may need to use a channel to handle large batches
	c.Db.BatchInsertIssues(ctx, batchIssues).Exec(func(i int, err error) {
		if err != nil {
			logrus.Printf("Failed to insert issue %d: %v", i, err)
		} else {
			logrus.Printf("Successfully inserted issue %d", i)
		}
	})

	// TODO: count and handle batch insert errors
	return nil
}

func Map[T any, U any](input []T, f func(T) U) []U {
	output := make([]U, len(input))
	for i, v := range input {
		output[i] = f(v)
	}
	return output
}
