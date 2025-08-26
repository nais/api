package checker

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/nais/api/internal/issue/checker/checkersql"
	"log"

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
	IssueTypeAivenIssue       IssueType = "AIVEN_ISSUE"
	IssueTypeSQLInstanceIssue IssueType = "SQLINSTANCE_ISSUE"
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
}

type KubernetesLister[T any] interface {
	List(context.Context) []T
}

type Config struct {
	AivenToken    string
	AivenProjects []string
}

func New(config Config, pool *pgxpool.Pool) *IssueChecker {
	return &IssueChecker{
		Config:            config,
		Db:                checkersql.New(pool),
		SQLInstanceLister: &SQLInstanceLister{},
	}
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
		AivenCheck{AivenClient: c, Projects: i.Config.AivenProjects},
		SQLInstanceCheck{SQLInstanceClient: sqladmin.Instances, SQLInstanceLister: i.SQLInstanceLister},
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
		fmt.Errorf("failed to delete existing issues: %w", err)
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
