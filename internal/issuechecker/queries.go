package issuechecker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/issuechecker/issuecheckersql"
)

const depKey ctxKey = iota

type ctxKey int

type dependencies struct {
	db *issuecheckersql.Queries
}

func NewContext(ctx context.Context, dbConn *pgxpool.Pool) context.Context {
	return context.WithValue(ctx, depKey, &dependencies{db: issuecheckersql.New(dbConn)})
}

func fromContext(ctx context.Context) *dependencies {
	return ctx.Value(depKey).(*dependencies)
}

func GetByIdent(ctx context.Context, id ident.Ident) (*Issue, error) {
	return nil, nil
}

func GetIssuesForTeam(ctx context.Context, teamSlug string) ([]*Issue, error) {
	issues, err := db(ctx).ListIssuesForTeam(ctx, teamSlug)
	if err != nil {
		return nil, err
	}

	ret := make([]*Issue, 0, len(issues))
	for _, issue := range issues {
		details, err := details(issue.IssueType, issue.IssueDetails)
		if err != nil {
			return nil, fmt.Errorf("unmarshal issue details: %w", err)
		}
		ret = append(ret, &Issue{
			ID:           newIdent(issue.ID.String()),
			Team:         issue.Team,
			ResourceName: issue.ResourceName,
			ResourceType: issue.ResourceType,
			Severity:     Severity(issue.Severity),
			Environment:  issue.Env,
			IssueType:    IssueType(issue.IssueType),
			Details:      details,
		})
	}
	return ret, nil
}

func details(issueType string, data []byte) (IssueDetails, error) {
	switch IssueType(issueType) {
	case IssueTypeAivenAlert:
		return unmarshal[AivenAlertDetails](data)
	case IssueTypeCloudSQL:
		return unmarshal[SQLInstanceStateDetails](data)
	default:
		return nil, fmt.Errorf("unknown issue type: %s", issueType)
	}
}

func db(ctx context.Context) *issuecheckersql.Queries {
	return fromContext(ctx).db
}

func unmarshal[T any](data []byte) (*T, error) {
	var t T
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	return &t, nil
}
