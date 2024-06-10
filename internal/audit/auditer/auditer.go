package auditer

import (
	"context"

	"github.com/nais/api/internal/auditevent"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/slug"
)

type Auditer struct {
	db database.AuditEventsRepo
}

func NewAuditer(db database.Database) *Auditer {
	return &Auditer{db: db}
}

func (a *Auditer) TeamAddMember(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug, memberEmail, role string) error {
	return a.db.CreateAuditEvent(ctx, auditevent.NewTeamAddMember(actor, team, memberEmail, role))
}

func (a *Auditer) TeamRemoveMember(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug, memberEmail string) error {
	return a.db.CreateAuditEvent(ctx, auditevent.NewTeamRemoveMember(actor, team, memberEmail))
}

func (a *Auditer) TeamSetMemberRole(ctx context.Context, actor authz.AuthenticatedUser, team slug.Slug, memberEmail string, role string) error {
	return a.db.CreateAuditEvent(ctx, auditevent.NewTeamSetMemberRole(actor, team, memberEmail, role))
}
