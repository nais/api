package team

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/team/teamsql"
	"k8s.io/utils/ptr"
)

func Create(ctx context.Context, input *CreateTeamInput, actor *authz.Actor) (*Team, error) {
	if err := input.Validate(ctx); err != nil {
		return nil, err
	}

	var team *teamsql.Team
	err := database.Transaction(ctx, func(ctx context.Context) error {
		var err error
		team, err = db(ctx).Create(ctx, teamsql.CreateParams{
			Slug:         input.Slug,
			Purpose:      input.Purpose,
			SlackChannel: input.SlackChannel,
		})
		if err != nil {
			return err
		}

		if !actor.User.IsServiceAccount() {
			err = authz.MakeUserTeamOwner(ctx, actor.User.GetID(), input.Slug)
		}
		if err != nil {
			return err
		}

		return activitylog.Create(ctx, activitylog.CreateInput{
			Action:       activitylog.ActivityLogEntryActionCreated,
			Actor:        actor.User,
			ResourceType: activityLogEntryResourceTypeTeam,
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
	if err := input.Validate(); err != nil {
		return nil, err
	}

	existingTeam, err := Get(ctx, input.Slug)
	if err != nil {
		return nil, err
	}

	if input.Purpose == nil && input.SlackChannel == nil {
		return existingTeam, nil
	}

	var team *teamsql.Team
	err = database.Transaction(ctx, func(ctx context.Context) error {
		team, err = db(ctx).Update(ctx, teamsql.UpdateParams{
			Purpose:      input.Purpose,
			SlackChannel: input.SlackChannel,
			Slug:         input.Slug,
		})
		if err != nil {
			return err
		}

		updatedFields := make([]*TeamUpdatedActivityLogEntryDataUpdatedField, 0)
		if input.Purpose != nil && *input.Purpose != existingTeam.Purpose {
			updatedFields = append(updatedFields, &TeamUpdatedActivityLogEntryDataUpdatedField{
				Field:    "purpose",
				OldValue: &existingTeam.Purpose,
				NewValue: input.Purpose,
			})
		}

		if input.SlackChannel != nil && *input.SlackChannel != existingTeam.SlackChannel {
			updatedFields = append(updatedFields, &TeamUpdatedActivityLogEntryDataUpdatedField{
				Field:    "slackChannel",
				OldValue: &existingTeam.SlackChannel,
				NewValue: input.SlackChannel,
			})
		}

		return activitylog.Create(ctx, activitylog.CreateInput{
			Action:       activitylog.ActivityLogEntryActionUpdated,
			Actor:        actor.User,
			ResourceType: activityLogEntryResourceTypeTeam,
			ResourceName: input.Slug.String(),
			TeamSlug:     ptr.To(input.Slug),
			Data: func(fields []*TeamUpdatedActivityLogEntryDataUpdatedField) *TeamUpdatedActivityLogEntryData {
				if len(fields) == 0 {
					return nil
				}

				return &TeamUpdatedActivityLogEntryData{
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
	t, err := fromContext(ctx).teamLoader.Load(ctx, slug)
	if err != nil {
		return nil, handleError(err)
	}
	return t, nil
}

func GetByIdent(ctx context.Context, id ident.Ident) (*Team, error) {
	teamSlug, err := parseTeamIdent(id)
	if err != nil {
		return nil, err
	}
	return Get(ctx, teamSlug)
}

func List(ctx context.Context, page *pagination.Pagination, orderBy *TeamOrder, filter *TeamFilter) (*TeamConnection, error) {
	if orderBy != nil && SortFilter.SupportsSort(orderBy.Field) {
		start := time.Now()
		defer func() {
			fmt.Println("Sorting teams took", time.Since(start))
		}()
		// These aren't available in the SQL database, so we need custom handling.
		return listAndSortByExternalSort(ctx, page, orderBy, filter)
	}

	q := db(ctx)

	ret, err := q.List(ctx, teamsql.ListParams{
		Offset:  page.Offset(),
		Limit:   page.Limit(),
		OrderBy: orderBy.String(),
	})
	if err != nil {
		return nil, err
	}

	teams := make([]*Team, len(ret))
	for i, t := range ret {
		teams[i] = toGraphTeam(&t.Team)
	}

	filteredTeams := SortFilter.Filter(ctx, teams, filter)

	if orderBy != nil {
		SortFilter.Sort(ctx, filteredTeams, orderBy.Field, orderBy.Direction)
	}

	return pagination.NewConnection(pagination.Slice(filteredTeams, page), page, len(filteredTeams)), nil
}

func listAndSortByExternalSort(ctx context.Context, page *pagination.Pagination, orderBy *TeamOrder, filter *TeamFilter) (*TeamConnection, error) {
	all, err := db(ctx).ListAllForExternalSort(ctx)
	if err != nil {
		return nil, err
	}

	teams := make([]*Team, len(all))
	for i, t := range all {
		teams[i] = toGraphTeam(t)
	}

	filteredTeams := SortFilter.Filter(ctx, teams, filter)

	SortFilter.Sort(ctx, filteredTeams, orderBy.Field, orderBy.Direction)

	return pagination.NewConnection(pagination.Slice(filteredTeams, page), page, len(filteredTeams)), nil
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

	var total int64
	if len(ret) > 0 {
		total = ret[0].TotalCount
	}
	return pagination.NewConvertConnection(ret, page, total, toGraphUserTeam), nil
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

	var total int64
	if len(ret) > 0 {
		total = ret[0].TotalCount
	}
	return pagination.NewConvertConnection(ret, page, total, toGraphTeamMember), nil
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
	err = database.Transaction(ctx, func(ctx context.Context) error {
		key, err = db(ctx).CreateDeleteKey(ctx, teamsql.CreateDeleteKeyParams{
			TeamSlug:  teamSlug,
			CreatedBy: actor.User.GetID(),
		})
		if err != nil {
			return err
		}

		return activitylog.Create(ctx, activitylog.CreateInput{
			Action:       activityLogEntryActionCreateDeleteKey,
			Actor:        actor.User,
			ResourceType: activityLogEntryResourceTypeTeam,
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
	return database.Transaction(ctx, func(ctx context.Context) error {
		db := db(ctx)

		if err := db.ConfirmDeleteKey(ctx, deleteKey); err != nil {
			return err
		}

		if err := db.SetDeleteKeyConfirmedAt(ctx, teamSlug); err != nil {
			return err
		}

		return activitylog.Create(ctx, activitylog.CreateInput{
			Action:       activityLogEntryActionConfirmDeleteKey,
			Actor:        actor.User,
			ResourceType: activityLogEntryResourceTypeTeam,
			ResourceName: teamSlug.String(),
			TeamSlug:     ptr.To(teamSlug),
		})
	})
}

func AddMember(ctx context.Context, input AddTeamMemberInput, actor *authz.Actor) error {
	_, err := db(ctx).GetMember(ctx, teamsql.GetMemberParams{
		TeamSlug: input.TeamSlug,
		UserID:   input.UserID,
	})
	if !errors.Is(err, pgx.ErrNoRows) {
		return apierror.Errorf("User is already a member of the team.")
	}

	return database.Transaction(ctx, func(ctx context.Context) error {
		params := teamsql.AddMemberParams{
			UserID:   input.UserID,
			RoleName: teamMemberRoleToSqlRole(input.Role),
			TeamSlug: input.TeamSlug,
		}
		if err := db(ctx).AddMember(ctx, params); err != nil {
			return err
		}

		return activitylog.Create(ctx, activitylog.CreateInput{
			Action:       activitylog.ActivityLogEntryActionAdded,
			Actor:        actor.User,
			ResourceType: activityLogEntryResourceTypeTeam,
			ResourceName: input.TeamSlug.String(),
			TeamSlug:     ptr.To(input.TeamSlug),
			Data: &TeamMemberAddedActivityLogEntryData{
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

	return database.Transaction(ctx, func(ctx context.Context) error {
		params := teamsql.RemoveMemberParams{
			UserID:   input.UserID,
			TeamSlug: input.TeamSlug,
		}
		if err := db(ctx).RemoveMember(ctx, params); err != nil {
			return err
		}

		return activitylog.Create(ctx, activitylog.CreateInput{
			Action:       activitylog.ActivityLogEntryActionRemoved,
			Actor:        actor.User,
			ResourceType: activityLogEntryResourceTypeTeam,
			ResourceName: input.TeamSlug.String(),
			TeamSlug:     ptr.To(input.TeamSlug),
			Data: &TeamMemberRemovedActivityLogEntryData{
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

	return database.Transaction(ctx, func(ctx context.Context) error {
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

		return activitylog.Create(ctx, activitylog.CreateInput{
			Action:       activityLogEntryActionSetMemberRole,
			Actor:        actor.User,
			ResourceType: activityLogEntryResourceTypeTeam,
			ResourceName: input.TeamSlug.String(),
			TeamSlug:     ptr.To(input.TeamSlug),
			Data: &TeamMemberSetRoleActivityLogEntryData{
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

func UpdateEnvironment(ctx context.Context, input *UpdateTeamEnvironmentInput, actor *authz.Actor) (*TeamEnvironment, error) {
	existingTeamEnvironment, err := db(ctx).GetEnvironment(ctx, teamsql.GetEnvironmentParams{
		Slug:        input.Slug,
		Environment: input.EnvironmentName,
	})
	if err != nil {
		return nil, err
	}

	if err := input.Validate(); err != nil {
		return nil, err
	}

	if input.SlackAlertsChannel == nil {
		return toGraphTeamEnvironment(existingTeamEnvironment), nil
	}

	err = database.Transaction(ctx, func(ctx context.Context) error {
		if input.SlackAlertsChannel != nil && *input.SlackAlertsChannel == "" {
			err = db(ctx).RemoveSlackAlertsChannel(ctx, teamsql.RemoveSlackAlertsChannelParams{
				TeamSlug:    input.Slug,
				Environment: input.EnvironmentName,
			})
		} else {
			err = db(ctx).UpsertEnvironment(ctx, teamsql.UpsertEnvironmentParams{
				TeamSlug:           input.Slug,
				Environment:        input.EnvironmentName,
				SlackAlertsChannel: input.SlackAlertsChannel,
				GcpProjectID:       input.GCPProjectID,
			})
		}
		if err != nil {
			return err
		}

		return activitylog.Create(ctx, activitylog.CreateInput{
			Action:       activityLogEntryActionUpdateEnvironment,
			Actor:        actor.User,
			ResourceType: activityLogEntryResourceTypeTeam,
			ResourceName: input.Slug.String(),
			TeamSlug:     ptr.To(input.Slug),
			Data: &TeamEnvironmentUpdatedActivityLogEntryData{
				UpdatedFields: []*TeamEnvironmentUpdatedActivityLogEntryDataUpdatedField{
					{
						Field:    "slackAlertsChannel",
						OldValue: &existingTeamEnvironment.SlackAlertsChannel,
						NewValue: input.SlackAlertsChannel,
					},
				},
			},
		})
	})
	if err != nil {
		return nil, err
	}

	te, err := db(ctx).GetEnvironment(ctx, teamsql.GetEnvironmentParams{
		Slug:        input.Slug,
		Environment: input.EnvironmentName,
	})
	if err != nil {
		return nil, err
	}

	return toGraphTeamEnvironment(te), nil
}

func Count(ctx context.Context) (int64, error) {
	// This is only implemented for vulnerability ranking. This should soon be removed.
	count, err := db(ctx).List(ctx, teamsql.ListParams{
		Limit: 1,
	})
	if err != nil {
		return 0, err
	}
	if len(count) == 0 {
		return 0, nil
	}

	return count[0].TotalCount, nil
}

// Exists checks if an active team with the given slug exists.
func Exists(ctx context.Context, slug slug.Slug) (bool, error) {
	return db(ctx).Exists(ctx, slug)
}

func UpdateExternalReferences(ctx context.Context, teamSlug slug.Slug, references *ExternalReferences) error {
	return db(ctx).UpdateExternalReferences(ctx, teamsql.UpdateExternalReferencesParams{
		Slug:             teamSlug,
		GoogleGroupEmail: references.GoogleGroupEmail,
		EntraIDGroupID:   references.EntraIDGroupID,
		GithubTeamSlug:   references.GithubTeamSlug,
		GarRepository:    references.GarRepository,
		CdnBucket:        references.CdnBucket,
	})
}

func ListBySlugs(ctx context.Context, slugs []slug.Slug, page *pagination.Pagination) (*TeamConnection, error) {
	ret, err := db(ctx).ListBySlugs(ctx, slugs)
	if err != nil {
		return nil, err
	}

	p := pagination.Slice(ret, page)
	return pagination.NewConvertConnection(p, page, len(ret), toGraphTeam), nil
}

func ListAllSlugs(ctx context.Context) ([]slug.Slug, error) {
	return db(ctx).ListAllSlugs(ctx)
}

func NamespaceExists(ctx context.Context, teamSlug slug.Slug) bool {
	watcher := fromContext(ctx).namespaceWatcher

	for _, r := range watcher.All() {
		if r.Obj.Name == teamSlug.String() {
			return true
		}
	}

	return false
}
