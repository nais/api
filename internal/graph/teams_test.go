package graph_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/api/internal/audit"
	"github.com/nais/api/internal/auditlogger"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/auth/roles"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/graph"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/loader"
	"github.com/nais/api/internal/slug"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestMutationResolver_RequestTeamDeletion(t *testing.T) {
	const tenant = "example"
	const tenantDomain = "example.com"
	log, _ := test.NewNullLogger()
	usersyncTrigger := make(chan<- uuid.UUID)
	ctx := context.Background()
	teamSlug := slug.Slug("my-team")

	t.Run("missing authz", func(t *testing.T) {
		db := database.NewMockDatabase(t)

		resolver := graph.
			NewResolver(nil, nil, nil, nil, db, tenant, tenantDomain, usersyncTrigger, auditlogger.NewAuditLoggerForTesting(), nil, nil, log, nil, nil, nil, nil, nil, nil, nil, nil).
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
		assert.ErrorContains(t, err, "required role: \"Team owner\"")
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
				TargetTeamSlug: &teamSlug,
				RoleName:       gensql.RoleNameTeamowner,
				Authorizations: []roles.Authorization{
					roles.AuthorizationTeamsMembersAdmin,
				},
			},
		})

		db := database.NewMockDatabase(t)
		db.EXPECT().
			GetTeamsBySlugs(mock.Anything, []slug.Slug{teamSlug}).Return(nil, nil).Once()

		ctx = loader.NewLoaderContext(ctx, db)
		resolver := graph.
			NewResolver(nil, nil, nil, nil, db, tenant, tenantDomain, usersyncTrigger, auditlogger.NewAuditLoggerForTesting(), nil, nil, log, nil, nil, nil, nil, nil, nil, nil, nil).
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
				TargetTeamSlug: &teamSlug,
				RoleName:       gensql.RoleNameTeamowner,
				Authorizations: []roles.Authorization{
					roles.AuthorizationTeamsMembersAdmin,
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

		db.EXPECT().CreateAuditEvent(mock.Anything, mock.Anything).Return(nil).Once()
		auditer := audit.NewAuditor(db)

		ctx = loader.NewLoaderContext(ctx, db)

		auditLogger := auditlogger.NewAuditLoggerForTesting()
		resolver := graph.
			NewResolver(nil, nil, nil, nil, db, tenant, tenantDomain, usersyncTrigger, auditLogger, nil, nil, log, nil, nil, nil, nil, nil, nil, nil, auditer).
			Mutation()

		returnedKey, err := resolver.RequestTeamDeletion(ctx, teamSlug)
		if err != nil {
			t.Fatal("unexpected error")
		}

		_ = returnedKey
	})
}
