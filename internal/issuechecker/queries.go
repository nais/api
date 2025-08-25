package issuechecker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/issuechecker/issuecheckersql"
)

const Key = "issuechecker"

func GetByIdent(ctx context.Context, id ident.Ident) (*Issue, error) {
	return nil, nil
}

func GetIssuesForTeam(ctx context.Context, teamSlug string) ([]*Issue, error) {
	issues, err := db(ctx).ListIssuesForTeam(ctx, teamSlug)
	if err != nil {
		return nil, err
	}

	result := make([]*Issue, 0, len(issues))
	for _, issue := range issues {
		details, err := details(issue.IssueType, issue.IssueDetails)
		if err != nil {
			return nil, fmt.Errorf("unmarshal issue details: %w", err)
		}
		result = append(result, &Issue{
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
	return result, nil
}

func details(issueType string, data []byte) (IssueDetails, error) {
	switch IssueType(issueType) {
	case IssueTypeAivenAlert:
		details := &AivenAlertDetails{}
		if err := json.Unmarshal(data, details); err != nil {
			return nil, err
		}
		return details, nil
	case IssueTypeCloudSQL:
		details := &SQLInstanceStateDetails{}
		if err := json.Unmarshal(data, details); err != nil {
			return nil, err
		}
		return details, nil
	default:
		return nil, fmt.Errorf("unknown issue type: %s", issueType)
	}
}

func db(ctx context.Context) *issuecheckersql.Queries {
	return ctx.Value(Key).(*issuecheckersql.Queries)
}
