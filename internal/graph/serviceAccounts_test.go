package graph_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/nais/api/internal/auditlogger"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/auth/roles"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/graph"
	"github.com/nais/api/internal/graph/model"
	"github.com/sirupsen/logrus/hooks/test"
)

func TestMutationResolver_Roles(t *testing.T) {
	serviceAccount := &database.ServiceAccount{
		ServiceAccount: &gensql.ServiceAccount{
			ID:   uuid.New(),
			Name: "User Name",
		},
	}
	ctx := authz.ContextWithActor(context.Background(), serviceAccount, []*authz.Role{
		{
			RoleName: gensql.RoleNameAdmin,
			Authorizations: []roles.Authorization{
				roles.AuthorizationTeamsCreate,
			},
		},
	})

	auditLogger := auditlogger.NewAuditLoggerForTesting()
	db := database.NewMockDatabase(t)
	log, _ := test.NewNullLogger()
	resolver := graph.
		NewResolver(nil, nil, nil, nil, db, "example", "example.com", auditLogger, nil, nil, log, nil, nil, nil, nil, nil, nil, nil, nil).
		ServiceAccount()

	t.Run("get roles for serviceAccount", func(t *testing.T) {
		role := &authz.Role{
			Authorizations:         nil,
			RoleName:               "",
			TargetServiceAccountID: &serviceAccount.ID,
			TargetTeamSlug:         nil,
		}

		db.EXPECT().
			GetServiceAccountRoles(ctx, serviceAccount.ID).
			Return([]*authz.Role{role}, nil)

		r, err := resolver.Roles(ctx, &model.ServiceAccount{
			ID: serviceAccount.ID,
		})
		if err != nil {
			t.Fatal("unexpected error")
		}
		if r[0].GQLVars.TargetServiceAccountID != *role.TargetServiceAccountID {
			t.Fatal("unexpected target service account id")
		}
	})
}
