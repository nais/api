package issuechecker

import (
	"context"

	aiven "github.com/aiven/go-client-codegen"
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
	Config Config
}

type Config struct {
	AivenToken    string
	AivenProjects []string
}

func New(config Config) *IssueChecker {
	return &IssueChecker{
		Config: config,
	}
}

func (i IssueChecker) RunChecks(ctx context.Context) {

	c, err := aiven.NewClient(aiven.TokenOpt(i.Config.AivenToken), aiven.UserAgentOpt("nais-api"))
	if err != nil {
		panic(err)
	}

	checks := []Check{
		AivenCheck{AivenClient: c, Projects: i.Config.AivenProjects},
	}

	var issues []Issue
	for _, check := range checks {
		checkIssues, err := check.Run(ctx)
		if err != nil {
			panic(err)
		}
		issues = append(issues, checkIssues...)
	}

	for _, issue := range issues {
		println("Found issue:", issue.ResourceName, "of type", issue.Type)
	}
}
