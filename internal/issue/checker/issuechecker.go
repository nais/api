package checker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/nais/api/internal/environment"
	"github.com/nais/api/internal/issue/checker/checkersql"
	"github.com/nais/api/internal/workload/application"

	aiven "github.com/aiven/go-client-codegen"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/persistence/sqlinstance"
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

type IssueChecker struct {
	Config            Config
	Db                checkersql.Querier
	SQLInstanceLister KubernetesLister[*sqlinstance.SQLInstance]
	applicationLister KubernetesLister[*application.Application]
	Environments      []string
}

type KubernetesLister[T any] interface {
	List(ctx context.Context) []T
}

type applicationLister struct {
	Environments []string
}

func New(config Config, pool *pgxpool.Pool) *IssueChecker {
	ctx := environment.NewLoaderContext(context.Background(), pool)
	envs, err := environment.List(ctx, nil)
	if err != nil {
		panic(fmt.Sprintf("failed to list environments: %v", err))
	}

	return &IssueChecker{
		Config:            config,
		Db:                checkersql.New(pool),
		SQLInstanceLister: &SQLInstanceLister{},
		applicationLister: &applicationLister{Environments: Map(envs, func(e *environment.Environment) string { return e.Name })},
	}
}

func (a *applicationLister) List(ctx context.Context) []*application.Application {
	ret := []*application.Application{}
	for _, env := range a.Environments {
		ret = append(ret, application.ListAllInEnvironment(ctx, env)...)
	}
	return ret
}

type Config struct {
	AivenToken    string
	AivenProjects []string
}

func (i IssueChecker) RunChecks(ctx context.Context) error {
	c, err := aiven.NewClient(aiven.TokenOpt(i.Config.AivenToken), aiven.UserAgentOpt("nais-api"))
	if err != nil {
		return err
	}

	sqladmin, err := sqladmin.NewService(ctx)
	if err != nil {
		return fmt.Errorf("failed to create SQL Admin service: %w", err)
	}

	checks := []Check{
		Aiven{AivenClient: c, Projects: i.Config.AivenProjects},
		SQLInstance{SQLInstanceClient: sqladmin.Instances, SQLInstanceLister: i.SQLInstanceLister},
		DeprecatedIngress{ApplicationLister: i.applicationLister, Environments: i.Environments},
	}

	var issues []Issue
	for _, check := range checks {
		checkIssues, err := check.Run(ctx)
		if err != nil {
			return fmt.Errorf("failed to run check: %w", err)
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
	err = i.Db.DeleteIssues(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete existing issues: %w", err)
	}

	// TODO: may need to use a channel to handle large batches
	i.Db.BatchInsertIssues(ctx, batchIssues).Exec(func(i int, err error) {
		if err != nil {
			log.Printf("Failed to insert issue %d: %v", i, err)
		} else {
			log.Printf("Successfully inserted issue %d", i)
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
