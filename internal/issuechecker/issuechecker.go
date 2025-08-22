package issuechecker

import (
	"context"
	"encoding/json"
	"log"

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
	SQLInstanceLister KubernetesLister[*sqlinstance.SQLInstance]
}

type KubernetesLister[T any] interface {
	List(context.Context) []T
}

type Config struct {
	AivenToken    string
	AivenProjects []string
}

func New(config Config) *IssueChecker {
	return &IssueChecker{
		Config:            config,
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

	for _, issue := range issues {
		println("Found issue:", issue.ResourceName, "of type", issue.Type)
		d, err := json.MarshalIndent(issue.IssueData, "", "  ")
		if err != nil {
			panic(err)
		}
		println("Issue data:", string(d))
	}
	// TODO: store issues in a database
}
