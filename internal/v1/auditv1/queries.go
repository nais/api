package auditv1

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/auditv1/auditsql"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"k8s.io/utils/ptr"
)

type AuditEventInput interface {
	GetAction() string
	GetActor() string
	GetData() any
	GetEnvironmentName() *string
	GetResourceType() string
	GetResourceName() string
	GetTeamSlug() *slug.Slug
}

func Create(ctx context.Context, event AuditEventInput) error {
	q := db(ctx)
	var data []byte
	if event.GetData() != nil {
		var err error

		data, err = json.Marshal(event.GetData())
		if err != nil {
			return err
		}
	}

	return q.Create(ctx, auditsql.CreateParams{
		Action:          event.GetAction(),
		Actor:           event.GetActor(),
		Data:            data,
		EnvironmentName: event.GetEnvironmentName(),
		ResourceName:    event.GetResourceName(),
		ResourceType:    event.GetResourceType(),
		TeamSlug:        event.GetTeamSlug(),
	})
}

func Get(ctx context.Context, uid uuid.UUID) (AuditLog, error) {
	return fromContext(ctx).auditLogLoader.Load(ctx, uid)
}

func GetByIdent(ctx context.Context, id ident.Ident) (AuditLog, error) {
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

func toGraphAuditLog(row *auditsql.AuditEvent) AuditLog {
	entry := AuditLogGeneric{
		Action:          AuditLogAction(row.Action),
		Actor:           row.Actor,
		CreatedAt:       row.CreatedAt.Time,
		EnvironmentName: row.Environment,
		ResourceType:    AuditLogResourceType(row.ResourceType),
		ResourceName:    row.ResourceName,
		TeamSlug:        row.TeamSlug,
		UUID:            row.ID,
	}

	entry = entry.WithMessage(cases.Title(language.English).String(row.Action + " " + row.ResourceType))

	transformer, ok := knownTransformers[AuditLogResourceType(row.ResourceType)]
	if ok {
		return transformer(entry)
	}

	return entry
}
