package issue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/issue/checker"
	"github.com/nais/api/internal/issue/issuesql"
)

const depKey ctxKey = iota

type ctxKey int

type dependencies struct {
	db *issuesql.Queries
}

func NewContext(ctx context.Context, dbConn *pgxpool.Pool) context.Context {
	return context.WithValue(ctx, depKey, &dependencies{db: issuesql.New(dbConn)})
}

func fromContext(ctx context.Context) *dependencies {
	return ctx.Value(depKey).(*dependencies)
}

func GetByIdent(ctx context.Context, id ident.Ident) (Issue, error) {
	return nil, nil
}

func GetIssuesForTeam(ctx context.Context, teamSlug string) ([]Issue, error) {
	issues, err := db(ctx).ListIssuesForTeam(ctx, teamSlug)
	if err != nil {
		return nil, err
	}

	ret := make([]Issue, 0, len(issues))
	for _, issue := range issues {
		i, err := convert(issue)
		if err != nil {
			return nil, fmt.Errorf("unmarshal issue details: %w", err)
		}
		ret = append(ret, i)
	}
	return ret, nil
}

func convert(issue *issuesql.Issue) (Issue, error) {
	switch checker.IssueType(issue.IssueType) {
	case checker.IssueTypeAivenIssue:
		d, err := unmarshal[checker.AivenIssueDetails](issue.IssueDetails)
		if err != nil {
			return nil, err
		}
		return &AivenIssue{
			ID:           newIdent(issue.ID.String()),
			ResourceName: issue.ResourceName,
			ResourceType: issue.ResourceType,
			Environment:  issue.Env,
			Team:         issue.Team,
			Severity:     Severity(issue.Severity),
			Message:      d.Message,
		}, nil
	case checker.IssueTypeSQLInstanceIssue:
		d, err := unmarshal[checker.SQLInstanceIssueDetails](issue.IssueDetails)
		if err != nil {
			return nil, err
		}
		return &SQLInstanceIssue{
			Environment:  issue.Env,
			ID:           newIdent(issue.ID.String()),
			Message:      d.Message,
			ResourceName: issue.ResourceName,
			ResourceType: issue.ResourceType,
			Severity:     Severity(issue.Severity),
			State:        SQLInstanceIssueState(d.State),
			Team:         issue.Team,
		}, nil
	case checker.IssueTypeDeprecatedIngress:
		d, err := unmarshal[checker.DeprecatedIngressIssueDetails](issue.IssueDetails)
		if err != nil {
			return nil, err
		}
		return &DeprecatedIngressIssue{
			ID:           newIdent(issue.ID.String()),
			ResourceName: issue.ResourceName,
			ResourceType: issue.ResourceType,
			Environment:  issue.Env,
			Team:         issue.Team,
			Severity:     Severity(issue.Severity),
			Ingresses:    d.Ingresses,
		}, nil

	default:
		return nil, fmt.Errorf("unknown issue type: %s", issue.IssueType)
	}
}

func db(ctx context.Context) *issuesql.Queries {
	return fromContext(ctx).db
}

func unmarshal[T any](data []byte) (*T, error) {
	var t T
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	return &t, nil
}
