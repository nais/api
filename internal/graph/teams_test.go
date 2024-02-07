package graph_test

import (
	"context"
	"testing"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/pubsub/pstest"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/api/internal/auditlogger"
	"github.com/nais/api/internal/auditlogger/audittype"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/auth/roles"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/graph"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/loader"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/logger"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/usersync"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestMutationResolver_CreateTeam(t *testing.T) {
	user := database.User{
		User: &gensql.User{
			ID:    uuid.New(),
			Email: "user@example.com",
			Name:  "User Name",
		},
	}

	ctx := authz.ContextWithActor(context.Background(), user, []*authz.Role{
		{
			RoleName: gensql.RoleNameAdmin,
			Authorizations: []roles.Authorization{
				roles.AuthorizationTeamsCreate,
			},
		},
	})

	serviceAccount := database.ServiceAccount{
		ServiceAccount: &gensql.ServiceAccount{
			ID:   uuid.New(),
			Name: "User Name",
		},
	}

	saCtx := authz.ContextWithActor(context.Background(), serviceAccount, []*authz.Role{
		{
			RoleName: gensql.RoleNameAdmin,
			Authorizations: []roles.Authorization{
				roles.AuthorizationTeamsCreate,
			},
		},
	})

	userSyncRuns := usersync.NewRunsHandler(5)
	db := database.NewMockDatabase(t)

	log, err := logger.New("text", "info")
	assert.NoError(t, err)
	userSync := make(chan<- uuid.UUID)
	teamSlug := slug.Slug("some-slug")
	slackChannel := "#my-slack-channel"
	const tenantDomain = "example.com"

	t.Run("create team with empty purpose", func(t *testing.T) {
		_, err := graph.
			NewResolver(nil, nil, nil, nil, db, tenantDomain, userSync, auditlogger.NewAuditLoggerForTesting(), nil, userSyncRuns, nil, log).
			Mutation().
			CreateTeam(ctx, model.CreateTeamInput{
				Slug:         teamSlug,
				Purpose:      "  ",
				SlackChannel: slackChannel,
			})
		assert.ErrorContains(t, err, "You must specify the purpose for your team")
	})

	t.Run("create team", func(t *testing.T) {
		createdTeam := &database.Team{
			Team: &gensql.Team{Slug: teamSlug},
		}
		txCtx := context.Background()
		dbtx := database.NewMockDatabase(t)
		dbtx.EXPECT().
			CreateTeam(txCtx, teamSlug, "some purpose", slackChannel).
			Return(createdTeam, nil).
			Once()
		dbtx.EXPECT().
			SetTeamMemberRole(txCtx, user.ID, createdTeam.Slug, gensql.RoleNameTeamowner).
			Return(nil).
			Once()

		db.
			EXPECT().
			Transaction(ctx, mock.Anything).
			Run(func(_ context.Context, fn database.DatabaseTransactionFunc) {
				_ = fn(txCtx, dbtx)
			}).
			Return(nil).
			Once()

		_, psClient, closer := getPubsubServerAndClient(ctx, "some-id", "topic-id")
		defer closer()

		// TODO: check message sent to the pubsub topic

		auditLogger := auditlogger.NewAuditLoggerForTesting()
		returnedTeam, err := graph.
			NewResolver(nil, nil, nil, nil, db, tenantDomain, userSync, auditLogger, nil, userSyncRuns, psClient.Topic("topic-id"), log).
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
		createdTeam := &database.Team{
			Team: &gensql.Team{Slug: teamSlug},
		}
		txCtx := context.Background()
		dbtx := database.NewMockDatabase(t)

		dbtx.EXPECT().
			CreateTeam(txCtx, teamSlug, "some purpose", slackChannel).
			Return(createdTeam, nil).
			Once()

		dbtx.EXPECT().
			AssignTeamRoleToServiceAccount(txCtx, serviceAccount.GetID(), gensql.RoleNameTeamowner, teamSlug).
			Return(nil).
			Once()

		db.
			EXPECT().
			Transaction(saCtx, mock.Anything).
			Run(func(_ context.Context, fn database.DatabaseTransactionFunc) {
				_ = fn(txCtx, dbtx)
			}).
			Return(nil).
			Once()

		_, psClient, closer := getPubsubServerAndClient(ctx, "some-id", "topic-id")
		defer closer()

		// TODO: check message sent to the pubsub topic

		auditLogger := auditlogger.NewAuditLoggerForTesting()
		returnedTeam, err := graph.
			NewResolver(nil, nil, nil, nil, db, tenantDomain, userSync, auditLogger, nil, userSyncRuns, psClient.Topic("topic-id"), log).
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
	log, _ := test.NewNullLogger()
	userSync := make(chan<- uuid.UUID)
	ctx := context.Background()
	teamSlug := slug.Slug("my-team")
	userSyncRuns := usersync.NewRunsHandler(5)

	t.Run("service accounts can not create delete keys", func(t *testing.T) {
		db := database.NewMockDatabase(t)

		resolver := graph.
			NewResolver(nil, nil, nil, nil, db, tenantDomain, userSync, auditlogger.NewAuditLoggerForTesting(), nil, userSyncRuns, nil, log).
			Mutation()

		serviceAccount := database.ServiceAccount{
			ServiceAccount: &gensql.ServiceAccount{
				ID:   uuid.New(),
				Name: "service-account",
			},
		}

		ctx := authz.ContextWithActor(ctx, serviceAccount, []*authz.Role{})
		key, err := resolver.RequestTeamDeletion(ctx, teamSlug)
		assert.Nil(t, key)
		assert.ErrorContains(t, err, "Service accounts are not allowed")
	})

	t.Run("missing authz", func(t *testing.T) {
		db := database.NewMockDatabase(t)

		resolver := graph.
			NewResolver(nil, nil, nil, nil, db, tenantDomain, userSync, auditlogger.NewAuditLoggerForTesting(), nil, userSyncRuns, nil, log).
			Mutation()

		user := database.User{
			User: &gensql.User{
				ID:    uuid.New(),
				Email: "user@example.com",
				Name:  "User Name",
			},
		}
		ctx := authz.ContextWithActor(ctx, user, []*authz.Role{})

		key, err := resolver.RequestTeamDeletion(ctx, teamSlug)
		assert.Nil(t, key)
		assert.ErrorContains(t, err, "required authorization")
	})

	t.Run("missing team", func(t *testing.T) {
		user := database.User{
			User: &gensql.User{
				ID:    uuid.New(),
				Email: "user@example.com",
				Name:  "User Name",
			},
		}
		ctx := authz.ContextWithActor(ctx, user, []*authz.Role{
			{
				RoleName: gensql.RoleNameTeamowner,
				Authorizations: []roles.Authorization{
					roles.AuthorizationTeamsUpdate,
				},
			},
		})

		db := database.NewMockDatabase(t)
		db.EXPECT().
			GetTeamsBySlugs(mock.Anything, []slug.Slug{teamSlug}).Return(nil, nil).Once()

		ctx = loader.NewLoaderContext(ctx, db)
		resolver := graph.
			NewResolver(nil, nil, nil, nil, db, tenantDomain, userSync, auditlogger.NewAuditLoggerForTesting(), nil, userSyncRuns, nil, log).
			Mutation()
		key, err := resolver.RequestTeamDeletion(ctx, teamSlug)
		assert.Nil(t, key)
		assert.ErrorIs(t, err, apierror.ErrTeamNotExist)
	})

	t.Run("create key", func(t *testing.T) {
		userID := uuid.New()
		user := database.User{
			User: &gensql.User{
				ID:    userID,
				Email: "user@example.com",
				Name:  "User Name",
			},
		}
		team := &database.Team{
			Team: &gensql.Team{
				Slug:         teamSlug,
				SlackChannel: "#channel",
			},
		}
		ctx := authz.ContextWithActor(ctx, user, []*authz.Role{
			{
				RoleName: gensql.RoleNameTeamowner,
				Authorizations: []roles.Authorization{
					roles.AuthorizationTeamsUpdate,
				},
			},
		})

		key := &database.TeamDeleteKey{
			TeamDeleteKey: &gensql.TeamDeleteKey{
				Key:         uuid.New(),
				TeamSlug:    teamSlug,
				CreatedAt:   pgtype.Timestamptz{},
				CreatedBy:   uuid.UUID{},
				ConfirmedAt: pgtype.Timestamptz{},
			},
		}

		db := database.NewMockDatabase(t)
		db.
			EXPECT().
			GetTeamsBySlugs(mock.Anything, []slug.Slug{teamSlug}).
			Return([]*database.Team{team}, nil).
			Once()
		db.
			EXPECT().
			CreateTeamDeleteKey(mock.Anything, teamSlug, userID).
			Return(key, nil).
			Once()

		ctx = loader.NewLoaderContext(ctx, db)

		auditLogger := auditlogger.NewAuditLoggerForTesting()
		resolver := graph.
			NewResolver(nil, nil, nil, nil, db, tenantDomain, userSync, auditLogger, nil, userSyncRuns, nil, log).
			Mutation()

		returnedKey, err := resolver.RequestTeamDeletion(ctx, teamSlug)
		if err != nil {
			t.Fatal("unexpected error")
		}

		_ = returnedKey

		assert.Len(t, auditLogger.Entries(), 1)

		entry := auditLogger.Entries()[0]
		target := entry.Targets[0]

		if ctx != entry.Context {
			t.Errorf("incorrect context in audit log entry")
		}

		if string(teamSlug) != target.Identifier {
			t.Errorf("incorrect target in audit log entry")
		}

		if audittype.AuditLogsTargetTypeTeam != target.Type {
			t.Errorf("incorrect target type in audit log entry")
		}

		if audittype.AuditActionGraphqlApiTeamsRequestDelete != entry.Fields.Action {
			t.Errorf("incorrect action in audit log entry")
		}

		if user.ID != entry.Fields.Actor.User.GetID() {
			t.Errorf("incorrect actor in audit log entry")
		}
	})
}

func getPubsubServerAndClient(ctx context.Context, projectID string, topics ...string) (*pstest.Server, *pubsub.Client, func()) {
	srv := pstest.NewServer()
	client, _ := pubsub.NewClient(
		ctx,
		projectID,
		option.WithEndpoint(srv.Addr),
		option.WithoutAuthentication(),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	)

	for _, topic := range topics {
		_, _ = client.CreateTopic(ctx, topic)
	}

	return srv, client, func() {
		_ = srv.Close()
		_ = client.Close()
	}
}
