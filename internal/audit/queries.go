package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/nais/api/internal/audit/auditsql"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"k8s.io/utils/ptr"
)

type CreateInput struct {
	Action       AuditAction
	Actor        authz.AuthenticatedUser
	ResourceType AuditResourceType
	ResourceName string

	Data            any        // optional
	EnvironmentName *string    // optional
	TeamSlug        *slug.Slug // optional
}

// MarshalData marshals audit entry data. Its inverse is UnmarshalData.
func MarshalData(input CreateInput) ([]byte, error) {
	if input.Data == nil {
		return nil, nil
	}

	bytes, err := json.Marshal(input.Data)
	if err != nil {
		return nil, fmt.Errorf("marshaling audit entry data: %w", err)
	}

	return bytes, nil
}

func Create(ctx context.Context, input CreateInput) error {
	q := db(ctx)

	data, err := MarshalData(input)
	if err != nil {
		return err
	}

	return q.Create(ctx, auditsql.CreateParams{
		Action:          string(input.Action),
		Actor:           input.Actor.Identity(),
		Data:            data,
		EnvironmentName: input.EnvironmentName,
		ResourceName:    input.ResourceName,
		ResourceType:    string(input.ResourceType),
		TeamSlug:        input.TeamSlug,
	})
}

func Get(ctx context.Context, uid uuid.UUID) (AuditEntry, error) {
	return fromContext(ctx).auditLogLoader.Load(ctx, uid)
}

func GetByIdent(ctx context.Context, id ident.Ident) (AuditEntry, error) {
	uid, err := parseIdent(id)
	if err != nil {
		return nil, err
	}
	return Get(ctx, uid)
}

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination) (*AuditEntryConnection, error) {
	q := db(ctx)

	ret, err := q.ListForTeam(ctx, auditsql.ListForTeamParams{
		TeamSlug: ptr.To(teamSlug),
		Offset:   page.Offset(),
		Limit:    page.Limit(),
	})
	if err != nil {
		return nil, err
	}

	total, err := q.CountForTeam(ctx, &teamSlug)
	if err != nil {
		return nil, err
	}
	return pagination.NewConvertConnectionWithError(ret, page, total, toGraphAuditLog)
}

var titler = cases.Title(language.English)

func toGraphAuditLog(row *auditsql.AuditEvent) (AuditEntry, error) {
	entry := GenericAuditEntry{
		Action:          AuditAction(row.Action),
		Actor:           row.Actor,
		CreatedAt:       row.CreatedAt.Time,
		EnvironmentName: row.Environment,
		Message:         titler.String(row.Action) + " " + titler.String(row.ResourceType),
		ResourceType:    AuditResourceType(row.ResourceType),
		ResourceName:    row.ResourceName,
		TeamSlug:        row.TeamSlug,
		UUID:            row.ID,
		Data:            row.Data,
	}

	transformer, ok := knownTransformers[AuditResourceType(row.ResourceType)]
	if !ok {
		return nil, fmt.Errorf("no transformer registered for audit resource type: %q", row.ResourceType)
	}

	return transformer(entry)
}
