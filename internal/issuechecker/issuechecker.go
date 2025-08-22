package issuechecker

import (
	"context"
	"encoding/json"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/issuechecker/issuecheckersql"

	aiven "github.com/aiven/go-client-codegen"
	"github.com/nais/api/internal/persistence/sqlinstance"
	"google.golang.org/api/sqladmin/v1"
)

type Issue struct {
	// identifiers
	ResourceName string
	ResourceType string
	Environment  string
	Team         string

	Severity  Severity
	Type      IssueType
	IssueData any
}

type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityTodo    Severity = "todo"
)

type IssueType string

type Check interface {
	Run(ctx context.Context) ([]Issue, error)
}

type IssueChecker struct {
	Config            Config
	Db                issuecheckersql.Querier
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
		Db:                issuecheckersql.New(pool),
		SQLInstanceLister: &SQLInstanceLister{},
	}
}

func (i IssueChecker) RunChecks(ctx context.Context) {
	c, err := aiven.NewClient(aiven.TokenOpt(i.Config.AivenToken), aiven.UserAgentOpt("nais-api"))
	if err != nil {
		panic(err)
	}

	sqladmin, err := sqladmin.NewService(ctx)
	if err != nil {
		log.Fatalf("Failed to create SQL Admin service: %v", err)
	}

	checks := []Check{
		AivenCheck{AivenClient: c, Projects: i.Config.AivenProjects},
		SQLInstanceCheck{SQLInstanceClient: sqladmin.Instances, SQLInstanceLister: i.SQLInstanceLister},
	}

	var issues []Issue
	for _, check := range checks {
		checkIssues, err := check.Run(ctx)
		if err != nil {
			log.Fatalf("Failed to run check: %v", err)
		}
		issues = append(issues, checkIssues...)
	}

	batchIssues := make([]issuecheckersql.BatchInsertIssuesParams, 0)
	for _, issue := range issues {
		println("Found issue:", issue.ResourceName, "of type", issue.Type)
		// TODO: use regular marshalling instead of json.MarshalIndent for production code
		d, err := json.MarshalIndent(issue.IssueData, "", "  ")
		if err != nil {
			panic(err)
		}
		println("Issue data:", string(d))

		batchIssues = append(batchIssues, issuecheckersql.BatchInsertIssuesParams{
			IssueType:    string(issue.Type),
			ResourceName: issue.ResourceName,
			ResourceType: issue.ResourceType,
			Team:         issue.Team,
			Env:          issue.Environment,
			Severity:     string(issue.Severity),
			IssueDetails: d,
		})
	}
	err = i.Db.DeleteIssues(ctx)
	if err != nil {
		log.Fatalf("Failed to delete existing issues: %v", err)
	}

	// TODO: may need to use a channel to handle large batches
	i.Db.BatchInsertIssues(ctx, batchIssues).Exec(func(i int, err error) {
		if err != nil {
			log.Printf("Failed to insert issue %d: %v", i, err)
		} else {
			log.Printf("Successfully inserted issue %d", i)
		}
	})
}
