package auditv1

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/auditv1/auditsql"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"k8s.io/utils/ptr"
)

type CreateInput struct {
	Action       AuditLogAction
	Actor        authz.AuthenticatedUser
	ResourceType AuditLogResourceType
	ResourceName string

	Data            any        // optional
	EnvironmentName *string    // optional
	TeamSlug        *slug.Slug // optional
}

func Create(ctx context.Context, input CreateInput) error {
	q := db(ctx)
	var data []byte
	if input.Data != nil {
		var err error

		data, err = json.Marshal(input.Data)
		if err != nil {
			return err
		}
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

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination) (*AuditLogConnection, error) {
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
	return pagination.NewConvertConnection(ret, page, int32(total), toGraphAuditLog), nil
}

var titler = cases.Title(language.English)

func toGraphAuditLog(row *auditsql.AuditEvent) AuditEntry {
	entry := AuditLogGeneric{
		Action:          AuditLogAction(row.Action),
		Actor:           row.Actor,
		CreatedAt:       row.CreatedAt.Time,
		EnvironmentName: row.Environment,
		Message:         titler.String(row.Action) + " " + titler.String(row.ResourceType),
		ResourceType:    AuditLogResourceType(row.ResourceType),
		ResourceName:    row.ResourceName,
		TeamSlug:        row.TeamSlug,
		UUID:            row.ID,
	}

	transformer, ok := knownTransformers[AuditLogResourceType(row.ResourceType)]
	if ok {
		return transformer(entry)
	}

	return entry
}
