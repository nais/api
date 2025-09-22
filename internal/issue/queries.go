package issue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/issue/issuesql"
	"github.com/nais/api/internal/persistence/sqlinstance"
	"github.com/nais/api/internal/slug"
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
	uid, err := uuid.Parse(id.String())
	if err != nil {
		return nil, err
	}
	issue, err := db(ctx).GetIssueByID(ctx, uid)
	if err != nil {
		return nil, err
	}

	return convert(issue)
}

func ListIssues(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination, orderBy *IssueOrder, filter *IssueFilter) (*pagination.Connection[Issue], error) {
	params := issuesql.ListIssuesParams{
		Team:    teamSlug.String(),
		Offset:  page.Offset(),
		Limit:   page.Limit(),
		OrderBy: orderBy.String(),
	}

	if filter != nil {
		params.Env = filter.Environments
		params.ResourceType = (*string)(filter.ResourceType)
		params.IssueType = (*string)(filter.IssueType)
		params.Severity = issuesql.NullSeverityLevel{SeverityLevel: issuesql.SeverityLevel(*filter.Severity), Valid: filter.Severity != nil}
		params.ResourceName = filter.ResourceName
	}

	ret, err := db(ctx).ListIssues(ctx, params)
	if err != nil {
		return nil, err
	}

	var total int64
	if len(ret) > 0 {
		total = ret[0].TotalCount
	}

	return pagination.NewConvertConnectionWithError(ret, page, total, func(from *issuesql.ListIssuesRow) (Issue, error) {
		iss := &issuesql.Issue{
			ID:           from.ID,
			IssueType:    from.IssueType,
			ResourceName: from.ResourceName,
			ResourceType: from.ResourceType,
			Team:         from.Team,
			Env:          from.Env,
			Severity:     from.Severity,
			Message:      from.Message,
			IssueDetails: from.IssueDetails,
			CreatedAt:    from.CreatedAt,
		}
		return convert(iss)
	})
}

func convert(issue *issuesql.Issue) (Issue, error) {
	base := Base{
		ID:              newIdent(issue.ID.String()),
		ResourceName:    issue.ResourceName,
		ResourceType:    ResourceType(issue.ResourceType),
		EnvironmentName: issue.Env,
		TeamSlug:        slug.Slug(issue.Team),
		Severity:        Severity(issue.Severity),
		Message:         issue.Message,
	}

	switch IssueType(issue.IssueType) {
	case IssueTypeOpenSearch:
		d, err := unmarshal[AivenIssueDetails](issue.IssueDetails)
		if err != nil {
			return nil, err
		}
		return &OpenSearchIssue{
			Base:  base,
			Event: d.Event,
		}, nil
	case IssueTypeValkey:
		d, err := unmarshal[AivenIssueDetails](issue.IssueDetails)
		if err != nil {
			return nil, err
		}
		return &ValkeyIssue{
			Base:  base,
			Event: d.Event,
		}, nil
	case IssueTypeSqlInstanceState:
		d, err := unmarshal[SQLInstanceIssueDetails](issue.IssueDetails)
		if err != nil {
			return nil, err
		}
		return &SqlInstanceStateIssue{
			Base:  base,
			State: sqlinstance.SQLInstanceState(d.State),
		}, nil
	case IssueTypeSqlInstanceVersion:
		return &SqlInstanceVersionIssue{
			Base: base,
		}, nil
	case IssueTypeDeprecatedRegistry:
		return &DeprecatedRegistryIssue{
			Base: base,
		}, nil
	case IssueTypeDeprecatedIngress:
		d, err := unmarshal[DeprecatedIngressIssueDetails](issue.IssueDetails)
		if err != nil {
			return nil, err
		}
		return &DeprecatedIngressIssue{
			Base:      base,
			Ingresses: d.Ingresses,
		}, nil
	case IssueTypeNoRunningInstances:
		return &NoRunningInstancesIssue{
			Base: base,
		}, nil
	case IssueTypeFailedJobRuns:
		return &FailedJobRunsIssue{
			Base: base,
		}, nil
	case IssueTypeInvalidSpec:
		return &InvalidSpecIssue{
			Base: base,
		}, nil
	case IssueTypeFailedSynchronization:
		return &FailedSynchronizationIssue{
			Base: base,
		}, nil
	case IssueTypeVulnerableImage:
		d, err := unmarshal[VulnerableImageIssueDetails](issue.IssueDetails)
		if err != nil {
			return nil, err
		}
		return &VulnerableImageIssue{
			Base:                        base,
			VulnerableImageIssueDetails: *d,
		}, nil
	case IssueTypeMissingSBOM:
		return &MissingSbomIssue{
			Base: base,
		}, nil
	}

	return nil, fmt.Errorf("unknown issue type: %s", issue.IssueType)
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
