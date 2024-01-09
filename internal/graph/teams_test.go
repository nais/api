package graph_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/auditlogger"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/auth/roles"
	db "github.com/nais/api/internal/database"
	sqlc "github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/graph"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/logger"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/usersync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestMutationResolver_CreateTeam(t *testing.T) {
	user := db.User{
		User: &sqlc.User{
			ID:    uuid.New(),
			Email: "user@example.com",
			Name:  "User Name",
		},
	}

	ctx := authz.ContextWithActor(context.Background(), user, []*db.Role{
		{
			RoleName: sqlc.RoleNameAdmin,
			Authorizations: []roles.Authorization{
				roles.AuthorizationTeamsCreate,
			},
		},
	})

	serviceAccount := db.ServiceAccount{
		ServiceAccount: &sqlc.ServiceAccount{
			ID:   uuid.New(),
			Name: "User Name",
		},
	}

	saCtx := authz.ContextWithActor(context.Background(), serviceAccount, []*db.Role{
		{
			RoleName: sqlc.RoleNameAdmin,
			Authorizations: []roles.Authorization{
				roles.AuthorizationTeamsCreate,
			},
		},
	})

	userSyncRuns := usersync.NewRunsHandler(5)
	database := db.NewMockDatabase(t)
	teamSyncHandler := teamsync.NewMockHandler(t)

	gcpEnvironments := []string{"env"}
	log, err := logger.GetLogger("text", "info")
	assert.NoError(t, err)
	userSync := make(chan<- uuid.UUID)
	teamSlug := slug.Slug("some-slug")
	slackChannel := "#my-slack-channel"
	const tenantDomain = "example.com"

	t.Run("create team with empty purpose", func(t *testing.T) {
		_, err := graph.
			NewResolver(teamSyncHandler, database, tenantDomain, userSync, auditlogger.NewAuditLoggerForTesting(), gcpEnvironments, log, userSyncRuns).
			Mutation().
			CreateTeam(ctx, model.CreateTeamInput{
				Slug:         teamSlug,
				Purpose:      "  ",
				SlackChannel: slackChannel,
			})
		assert.ErrorContains(t, err, "You must specify the purpose for your team")
	})

	t.Run("create team", func(t *testing.T) {
		createdTeam := &db.Team{
			Team: &sqlc.Team{Slug: teamSlug},
		}
		txCtx := context.Background()
		dbtx := db.NewMockDatabase(t)

		dbtx.
			On("CreateTeam", txCtx, teamSlug, "some purpose", slackChannel).
			Return(createdTeam, nil).
			Once()
		dbtx.
			On("SetTeamMemberRole", txCtx, user.ID, createdTeam.Slug, sqlc.RoleNameTeamowner).
			Return(nil).
			Once()

		database.
			On("Transaction", ctx, mock.Anything).
			Run(func(args mock.Arguments) {
				fn := args.Get(1).(db.DatabaseTransactionFunc)
				fn(txCtx, dbtx)
			}).
			Return(nil).
			Once()

		teamSyncHandler.
			On("Schedule", mock.MatchedBy(func(input teamsync.Input) bool {
				return input.TeamSlug == createdTeam.Slug
			})).
			Return(nil).
			Once()

		auditLogger := auditlogger.NewAuditLoggerForTesting()
		returnedTeam, err := graph.
			NewResolver(teamSyncHandler, database, tenantDomain, userSync, auditLogger, gcpEnvironments, log, userSyncRuns).
			Mutation().
			CreateTeam(ctx, model.CreateTeamInput{
				Slug:         teamSlug,
				Purpose:      " some purpose ",
				SlackChannel: slackChannel,
			})
		assert.NoError(t, err)
		assert.Equal(t, createdTeam.Slug, returnedTeam.Slug)
		assert.Len(t, auditLogger.Entries(), 1)
		entry := auditLogger.Entries()[0]
		assert.Equal(t, ctx, entry.Context)
		assert.Equal(t, string(createdTeam.Slug), entry.Targets[0].Identifier)
		assert.Equal(t, user, entry.Fields.Actor.User)
		assert.Equal(t, "Team created", entry.Message)
	})

	t.Run("calling with SA, adds sa as team owner", func(t *testing.T) {
		createdTeam := &db.Team{
			Team: &sqlc.Team{Slug: teamSlug},
		}
		txCtx := context.Background()
		dbtx := db.NewMockDatabase(t)

		dbtx.
			On("CreateTeam", txCtx, teamSlug, "some purpose", slackChannel).
			Return(createdTeam, nil).
			Once()

		dbtx.
			On("AssignTeamRoleToServiceAccount", txCtx, serviceAccount.GetID(), sqlc.RoleNameTeamowner, teamSlug).
			Return(nil).
			Once()

		database.
			On("Transaction", saCtx, mock.Anything).
			Run(func(args mock.Arguments) {
				fn := args.Get(1).(db.DatabaseTransactionFunc)
				fn(txCtx, dbtx)
			}).
			Return(nil).
			Once()

		teamSyncHandler.
			On("Schedule", mock.MatchedBy(func(input teamsync.Input) bool {
				return input.TeamSlug == createdTeam.Slug
			})).
			Return(nil).
			Once()

		auditLogger := auditlogger.NewAuditLoggerForTesting()
		returnedTeam, err := graph.
			NewResolver(teamSyncHandler, database, tenantDomain, userSync, auditLogger, gcpEnvironments, log, userSyncRuns).
			Mutation().CreateTeam(saCtx, model.CreateTeamInput{
			Slug:         teamSlug,
			Purpose:      " some purpose ",
			SlackChannel: slackChannel,
		})

		assert.NoError(t, err)
		assert.Equal(t, createdTeam.Slug, returnedTeam.Slug)
		assert.Len(t, auditLogger.Entries(), 1)
		entry := auditLogger.Entries()[0]
		assert.Equal(t, saCtx, entry.Context)
		assert.Equal(t, string(createdTeam.Slug), entry.Targets[0].Identifier)
		assert.Equal(t, serviceAccount, entry.Fields.Actor.User)
		assert.Equal(t, "Team created", entry.Message)
	})
}

func TestMutationResolver_RequestTeamDeletion(t *testing.T) {
	const tenantDomain = "example.com"
	teamSyncHandler := teamsync.NewMockHandler(t)
	database := db.NewMockDatabase(t)
	log := logger.NewMockLogger(t)
	log.
		On("WithComponent", types.ComponentNameGraphqlApi).
		Return(log)
	userSync := make(chan<- uuid.UUID)
	gcpEnvironments := []string{"env"}
	ctx := context.Background()
	teamSlug := slug.Slug("my-team")
	userSyncRuns := usersync.NewRunsHandler(5)

	t.Run("service accounts can not create delete keys", func(t *testing.T) {
		resolver := graph.
			NewResolver(teamSyncHandler, database, tenantDomain, userSync, auditlogger.NewAuditLoggerForTesting(), gcpEnvironments, log, userSyncRuns).
			Mutation()

		serviceAccount := db.ServiceAccount{
			ServiceAccount: &sqlc.ServiceAccount{
				ID:   uuid.New(),
				Name: "service-account",
			},
		}

		ctx := authz.ContextWithActor(ctx, serviceAccount, []*db.Role{})
		key, err := resolver.RequestTeamDeletion(ctx, teamSlug)
		assert.Nil(t, key)
		assert.ErrorContains(t, err, "Service accounts are not allowed")
	})

	t.Run("missing authz", func(t *testing.T) {
		resolver := graph.
			NewResolver(teamSyncHandler, database, tenantDomain, userSync, auditlogger.NewAuditLoggerForTesting(), gcpEnvironments, log, userSyncRuns).
			Mutation()

		user := db.User{
			User: &sqlc.User{
				ID:    uuid.New(),
				Email: "user@example.com",
				Name:  "User Name",
			},
		}
		ctx := authz.ContextWithActor(ctx, user, []*db.Role{})

		key, err := resolver.RequestTeamDeletion(ctx, teamSlug)
		assert.Nil(t, key)
		assert.ErrorContains(t, err, "required authorization")
	})

	t.Run("missing team", func(t *testing.T) {
		user := db.User{
			User: &sqlc.User{
				ID:    uuid.New(),
				Email: "user@example.com",
				Name:  "User Name",
			},
		}
		ctx := authz.ContextWithActor(ctx, user, []*db.Role{
			{
				RoleName: sqlc.RoleNameTeamowner,
				Authorizations: []roles.Authorization{
					roles.AuthorizationTeamsUpdate,
				},
			},
		})

		database := db.NewMockDatabase(t)
		database.
			On("GetTeamBySlug", ctx, teamSlug).
			Return(nil, fmt.Errorf("some error")).
			Once()

		resolver := graph.
			NewResolver(teamSyncHandler, database, tenantDomain, userSync, auditlogger.NewAuditLoggerForTesting(), gcpEnvironments, log, userSyncRuns).
			Mutation()

		key, err := resolver.RequestTeamDeletion(ctx, teamSlug)
		assert.Nil(t, key)
		assert.ErrorIs(t, err, apierror.ErrTeamNotExist)
	})

	t.Run("create key", func(t *testing.T) {
		userID := uuid.New()
		user := db.User{
			User: &sqlc.User{
				ID:    userID,
				Email: "user@example.com",
				Name:  "User Name",
			},
		}
		team := &db.Team{
			Team: &sqlc.Team{
				Slug:         teamSlug,
				SlackChannel: "#channel",
			},
		}
		ctx := authz.ContextWithActor(ctx, user, []*db.Role{
			{
				RoleName: sqlc.RoleNameTeamowner,
				Authorizations: []roles.Authorization{
					roles.AuthorizationTeamsUpdate,
				},
			},
		})

		key := &db.TeamDeleteKey{
			TeamDeleteKey: &sqlc.TeamDeleteKey{
				Key:         uuid.UUID{},
				TeamSlug:    teamSlug,
				CreatedAt:   time.Time{},
				CreatedBy:   uuid.UUID{},
				ConfirmedAt: nil,
			},
		}

		database := db.NewMockDatabase(t)
		database.
			On("GetTeamBySlug", ctx, teamSlug).
			Return(team, nil).
			Once()
		database.
			On("CreateTeamDeleteKey", ctx, teamSlug, userID).
			Return(key, nil).
			Once()

		auditLogger := auditlogger.NewAuditLoggerForTesting()
		resolver := graph.
			NewResolver(teamSyncHandler, database, tenantDomain, userSync, auditLogger, gcpEnvironments, log, userSyncRuns).
			Mutation()

		returnedKey, err := resolver.RequestTeamDeletion(ctx, teamSlug)
		assert.Equal(t, key, returnedKey)
		assert.NoError(t, err)

		assert.Len(t, auditLogger.Entries(), 1)
		entry := auditLogger.Entries()[0]
		assert.Equal(t, ctx, entry.Context)
		assert.Equal(t, string(teamSlug), entry.Targets[0].Identifier)
		assert.Equal(t, types.AuditLogsTargetTypeTeam, entry.Targets[0].Type)
		assert.Equal(t, types.AuditActionGraphqlApiTeamsRequestDelete, entry.Fields.Action)
		assert.Equal(t, user, entry.Fields.Actor.User)
		assert.Equal(t, "Request team deletion", entry.Message)
	})
}
