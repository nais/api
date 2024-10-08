package team

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/auditv1"
	"github.com/nais/api/internal/v1/databasev1"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/role"
	"github.com/nais/api/internal/v1/role/rolesql"
	"github.com/nais/api/internal/v1/searchv1"
	"github.com/nais/api/internal/v1/team/teamsql"
	"github.com/nais/api/internal/v1/validate"
	"k8s.io/utils/ptr"
)

func Create(ctx context.Context, input *CreateTeamInput, actor *authz.Actor) (*Team, error) {
	input = input.Sanitized()

	err := validate.Validate(input)
	if err != nil {
		return nil, err
	}

	exists, err := db(ctx).SlugAvailable(ctx, input.Slug)
	if err != nil {
		return nil, err
	}

	if exists {
		return nil, apierror.Errorf("Team slug %q is not available.", input.Slug)
	}

	var team *teamsql.Team
	err = databasev1.Transaction(ctx, func(ctx context.Context) error {
		team, err = db(ctx).Create(ctx, teamsql.CreateParams{
			Slug:         input.Slug,
			Purpose:      input.Purpose,
			SlackChannel: input.SlackChannel,
		})
		if err != nil {
			return err
		}

		if actor.User.IsServiceAccount() {
			return role.AssignTeamRoleToServiceAccount(ctx, actor.User.GetID(), input.Slug, rolesql.RoleNameTeamowner)
		}

		err := role.AssignTeamRoleToUser(ctx, actor.User.GetID(), input.Slug, rolesql.RoleNameTeamowner)
		if err != nil {
			return err
		}

		return auditv1.Create(ctx, auditv1.CreateInput{
			Action:       auditv1.AuditActionCreated,
			Actor:        actor.User,
			ResourceType: auditResourceTypeTeam,
			ResourceName: input.Slug.String(),
			TeamSlug:     ptr.To(input.Slug),
		})
	})
	if err != nil {
		return nil, err
	}

	return toGraphTeam(team), nil
}

func Update(ctx context.Context, input *UpdateTeamInput, actor *authz.Actor) (*Team, error) {
	existingTeam, err := Get(ctx, input.Slug)
	if err != nil {
		return nil, err
	}

	input = input.Sanitized()
	if err := validate.Validate(input); err != nil {
		return nil, err
	}

	var team *teamsql.Team
	err = databasev1.Transaction(ctx, func(ctx context.Context) error {
		team, err = db(ctx).Update(ctx, teamsql.UpdateParams{
			Purpose:      input.Purpose,
			SlackChannel: input.SlackChannel,
			Slug:         input.Slug,
		})
		if err != nil {
			return err
		}

		updatedFields := make([]*TeamUpdatedAuditEntryDataUpdatedField, 0)
		if input.Purpose != nil && *input.Purpose != existingTeam.Purpose {
			updatedFields = append(updatedFields, &TeamUpdatedAuditEntryDataUpdatedField{
				Field:    "purpose",
				OldValue: &existingTeam.Purpose,
				NewValue: input.Purpose,
			})
		}

		if input.SlackChannel != nil && *input.SlackChannel != existingTeam.SlackChannel {
			updatedFields = append(updatedFields, &TeamUpdatedAuditEntryDataUpdatedField{
				Field:    "slackChannel",
				OldValue: &existingTeam.SlackChannel,
				NewValue: input.SlackChannel,
			})
		}

		return auditv1.Create(ctx, auditv1.CreateInput{
			Action:       auditv1.AuditActionUpdated,
			Actor:        actor.User,
			ResourceType: auditResourceTypeTeam,
			ResourceName: input.Slug.String(),
			TeamSlug:     ptr.To(input.Slug),
			Data: func(fields []*TeamUpdatedAuditEntryDataUpdatedField) *TeamUpdatedAuditEntryData {
				if len(fields) == 0 {
					return nil
				}

				return &TeamUpdatedAuditEntryData{
					UpdatedFields: fields,
				}
			}(updatedFields),
		})
	})
	if err != nil {
		return nil, err
	}

	return toGraphTeam(team), nil
}

func Get(ctx context.Context, slug slug.Slug) (*Team, error) {
	return fromContext(ctx).teamLoader.Load(ctx, slug)
}

func GetByIdent(ctx context.Context, id ident.Ident) (*Team, error) {
	teamSlug, err := parseTeamIdent(id)
	if err != nil {
		return nil, err
	}
	return Get(ctx, teamSlug)
}

func List(ctx context.Context, page *pagination.Pagination, orderBy *TeamOrder) (*TeamConnection, error) {
	q := db(ctx)

	ret, err := q.List(ctx, teamsql.ListParams{
		Offset:  page.Offset(),
		Limit:   page.Limit(),
		OrderBy: orderBy.String(),
	})
	if err != nil {
		return nil, err
	}

	total, err := q.Count(ctx)
	if err != nil {
		return nil, err
	}
	return pagination.NewConvertConnection(ret, page, int32(total), toGraphTeam), nil
}

func ListForUser(ctx context.Context, userID uuid.UUID, page *pagination.Pagination, orderBy *UserTeamOrder) (*TeamMemberConnection, error) {
	q := db(ctx)

	ret, err := q.ListForUser(ctx, teamsql.ListForUserParams{
		UserID:  userID,
		Offset:  page.Offset(),
		Limit:   page.Limit(),
		OrderBy: orderBy.String(),
	})
	if err != nil {
		return nil, err
	}

	total, err := q.CountForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return pagination.NewConvertConnection(ret, page, int32(total), toGraphUserTeam), nil
}

func ListGCPGroupsForUser(ctx context.Context, userID uuid.UUID) ([]string, error) {
	return db(ctx).ListGCPGroupsForUser(ctx, userID)
}

func GetMemberByEmail(ctx context.Context, teamSlug slug.Slug, email string) (*TeamMember, error) {
	q := db(ctx)

	m, err := q.GetMemberByEmail(ctx, teamsql.GetMemberByEmailParams{
		TeamSlug: teamSlug,
		Email:    email,
	})
	if err != nil {
		return nil, err
	}
	return &TeamMember{
		Role:     teamMemberRoleFromSqlTeamRole(m.RoleName),
		TeamSlug: teamSlug,
		UserID:   m.ID,
	}, nil
}

func ListMembers(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination, orderBy *TeamMemberOrder) (*TeamMemberConnection, error) {
	q := db(ctx)

	ret, err := q.ListMembers(ctx, teamsql.ListMembersParams{
		TeamSlug: teamSlug,
		Offset:   page.Offset(),
		Limit:    page.Limit(),
		OrderBy:  orderBy.String(),
	})
	if err != nil {
		return nil, err
	}

	total, err := q.CountMembers(ctx, &teamSlug)
	if err != nil {
		return nil, err
	}
	return pagination.NewConvertConnection(ret, page, int32(total), toGraphTeamMember), nil
}

func GetTeamEnvironment(ctx context.Context, teamSlug slug.Slug, envName string) (*TeamEnvironment, error) {
	return fromContext(ctx).teamEnvironmentLoader.Load(ctx, envSlugName{Slug: teamSlug, EnvName: envName})
}

func GetTeamEnvironmentByIdent(ctx context.Context, id ident.Ident) (*TeamEnvironment, error) {
	teamSlug, envName, err := parseTeamEnvironmentIdent(id)
	if err != nil {
		return nil, err
	}
	return GetTeamEnvironment(ctx, teamSlug, envName)
}

func ListTeamEnvironments(ctx context.Context, teamSlug slug.Slug) ([]*TeamEnvironment, error) {
	tes, err := db(ctx).ListEnvironmentsBySlug(ctx, teamSlug)
	if err != nil {
		return nil, err
	}

	ret := make([]*TeamEnvironment, len(tes))
	for i, te := range tes {
		ret[i] = toGraphTeamEnvironment(te)
	}

	return ret, nil
}

func GetDeleteKey(ctx context.Context, teamSlug slug.Slug, key uuid.UUID) (*TeamDeleteKey, error) {
	ret, err := db(ctx).GetDeleteKey(ctx, teamsql.GetDeleteKeyParams{
		Key:  key,
		Slug: teamSlug,
	})
	if err != nil {
		return nil, err
	}

	return toGraphTeamDeleteKey(ret), nil
}

func CreateDeleteKey(ctx context.Context, teamSlug slug.Slug, actor *authz.Actor) (*TeamDeleteKey, error) {
	var key *teamsql.TeamDeleteKey
	var err error
	err = databasev1.Transaction(ctx, func(ctx context.Context) error {
		key, err = db(ctx).CreateDeleteKey(ctx, teamsql.CreateDeleteKeyParams{
			TeamSlug:  teamSlug,
			CreatedBy: actor.User.GetID(),
		})
		if err != nil {
			return err
		}

		return auditv1.Create(ctx, auditv1.CreateInput{
			Action:       auditActionCreateDeleteKey,
			Actor:        actor.User,
			ResourceType: auditResourceTypeTeam,
			ResourceName: teamSlug.String(),
			TeamSlug:     ptr.To(teamSlug),
		})
	})
	if err != nil {
		return nil, err
	}

	return toGraphTeamDeleteKey(key), nil
}

func ConfirmDeleteKey(ctx context.Context, teamSlug slug.Slug, deleteKey uuid.UUID, actor *authz.Actor) error {
	return databasev1.Transaction(ctx, func(ctx context.Context) error {
		db := db(ctx)

		if err := db.ConfirmDeleteKey(ctx, deleteKey); err != nil {
			return err
		}

		if err := db.SetDeleteKeyConfirmedAt(ctx, teamSlug); err != nil {
			return err
		}

		return auditv1.Create(ctx, auditv1.CreateInput{
			Action:       auditActionConfirmDeleteKey,
			Actor:        actor.User,
			ResourceType: auditResourceTypeTeam,
			ResourceName: teamSlug.String(),
			TeamSlug:     ptr.To(teamSlug),
		})
	})
}

func Search(ctx context.Context, q string) ([]*searchv1.Result, error) {
	ret, err := db(ctx).Search(ctx, q)
	if err != nil {
		return nil, err
	}

	results := make([]*searchv1.Result, len(ret))
	for i, team := range ret {
		results[i] = &searchv1.Result{
			Node: toGraphTeam(&team.Team),
			Rank: searchv1.Match(q, team.Team.Slug.String()),
		}
	}

	return results, nil
}

func AddMember(ctx context.Context, input AddTeamMemberInput, actor *authz.Actor) error {
	_, err := db(ctx).GetMember(ctx, teamsql.GetMemberParams{
		TeamSlug: input.TeamSlug,
		UserID:   input.UserID,
	})
	if !errors.Is(err, pgx.ErrNoRows) {
		return apierror.Errorf("User is already a member of the team.")
	}

	return databasev1.Transaction(ctx, func(ctx context.Context) error {
		params := teamsql.AddMemberParams{
			UserID:   input.UserID,
			RoleName: teamMemberRoleToSqlRole(input.Role),
			TeamSlug: input.TeamSlug,
		}
		if err := db(ctx).AddMember(ctx, params); err != nil {
			return err
		}

		return auditv1.Create(ctx, auditv1.CreateInput{
			Action:       auditActionAddMember,
			Actor:        actor.User,
			ResourceType: auditResourceTypeTeam,
			ResourceName: input.TeamSlug.String(),
			TeamSlug:     ptr.To(input.TeamSlug),
			Data: &TeamMemberAddedAuditEntryData{
				Role:      input.Role,
				UserUUID:  input.UserID,
				UserEmail: input.UserEmail,
			},
		})
	})
}

func RemoveMember(ctx context.Context, input RemoveTeamMemberInput, actor *authz.Actor) error {
	_, err := db(ctx).GetMember(ctx, teamsql.GetMemberParams{
		TeamSlug: input.TeamSlug,
		UserID:   input.UserID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return apierror.Errorf("User is not a member of the team.")
	} else if err != nil {
		return err
	}

	return databasev1.Transaction(ctx, func(ctx context.Context) error {
		params := teamsql.RemoveMemberParams{
			UserID:   input.UserID,
			TeamSlug: input.TeamSlug,
		}
		if err := db(ctx).RemoveMember(ctx, params); err != nil {
			return err
		}

		return auditv1.Create(ctx, auditv1.CreateInput{
			Action:       auditActionRemoveMember,
			Actor:        actor.User,
			ResourceType: auditResourceTypeTeam,
			ResourceName: input.TeamSlug.String(),
			TeamSlug:     ptr.To(input.TeamSlug),
			Data: &TeamMemberRemovedAuditEntryData{
				UserUUID:  input.UserID,
				UserEmail: input.UserEmail,
			},
		})
	})
}

func SetMemberRole(ctx context.Context, input SetTeamMemberRoleInput, actor *authz.Actor) error {
	m, err := db(ctx).GetMember(ctx, teamsql.GetMemberParams{
		TeamSlug: input.TeamSlug,
		UserID:   input.UserID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return apierror.Errorf("User is not a member of the team.")
	} else if err != nil {
		return err
	}

	roleName := teamMemberRoleToSqlRole(input.Role)
	if m.RoleName == roleName {
		return apierror.Errorf("Member already has the %q role.", input.Role)
	}

	return databasev1.Transaction(ctx, func(ctx context.Context) error {
		err := db(ctx).RemoveMember(ctx, teamsql.RemoveMemberParams{
			UserID:   input.UserID,
			TeamSlug: input.TeamSlug,
		})
		if err != nil {
			return err
		}

		err = db(ctx).AddMember(ctx, teamsql.AddMemberParams{
			UserID:   input.UserID,
			RoleName: roleName,
			TeamSlug: input.TeamSlug,
		})
		if err != nil {
			return err
		}

		return auditv1.Create(ctx, auditv1.CreateInput{
			Action:       auditActionSetMemberRole,
			Actor:        actor.User,
			ResourceType: auditResourceTypeTeam,
			ResourceName: input.TeamSlug.String(),
			TeamSlug:     ptr.To(input.TeamSlug),
			Data: &TeamMemberSetRoleAuditEntryData{
				Role:      input.Role,
				UserUUID:  input.UserID,
				UserEmail: input.UserEmail,
			},
		})
	})
}

func UserIsOwner(ctx context.Context, teamSlug slug.Slug, userID uuid.UUID) (bool, error) {
	return db(ctx).UserIsOwner(ctx, teamsql.UserIsOwnerParams{
		UserID:   userID,
		TeamSlug: teamSlug,
	})
}

func UserIsMember(ctx context.Context, teamSlug slug.Slug, userID uuid.UUID) (bool, error) {
	return db(ctx).UserIsMember(ctx, teamsql.UserIsMemberParams{
		UserID:   userID,
		TeamSlug: teamSlug,
	})
}
