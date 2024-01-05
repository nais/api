package fixtures_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/nais/api/internal/db"
	"github.com/nais/api/internal/fixtures"
	"github.com/nais/api/internal/sqlc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestSetupStaticServiceAccounts(t *testing.T) {
	serviceAccounts := make(fixtures.ServiceAccounts, 0)

	t.Run("empty string", func(t *testing.T) {
		err := serviceAccounts.Decode("")
		assert.NoError(t, err)
		assert.Len(t, serviceAccounts, 0)
	})

	t.Run("invalid data", func(t *testing.T) {
		err := serviceAccounts.Decode(`{"foo":"bar"}`)
		assert.ErrorContains(t, err, `json: cannot unmarshal object`)
		assert.Len(t, serviceAccounts, 0)
	})

	t.Run("service account with no roles", func(t *testing.T) {
		err := serviceAccounts.Decode(`[{
			"name": "nais-service-account",
			"apiKey": "some key",
			"roles": []
		}]`)
		assert.EqualError(t, err, `service account must have at least one role: "nais-service-account"`)
		assert.Len(t, serviceAccounts, 0)
	})

	t.Run("missing API key", func(t *testing.T) {
		err := serviceAccounts.Decode(`[{
			"name": "nais-service-account",
			"roles": [{"name":"Admin"}]
		}]`)
		assert.EqualError(t, err, `service account is missing an API key: "nais-service-account"`)
		assert.Len(t, serviceAccounts, 0)
	})

	t.Run("service account with invalid name", func(t *testing.T) {
		err := serviceAccounts.Decode(`[{
			"name": "service-account",
			"apiKey": "some key",
			"roles": [{"name":"Team viewer"}]
		}]`)
		assert.EqualError(t, err, `service account is missing required "nais-" prefix: "service-account"`)
		assert.Len(t, serviceAccounts, 0)
	})

	t.Run("service account with invalid role", func(t *testing.T) {
		err := serviceAccounts.Decode(`[{
			"name": "nais-service-account",
			"apiKey": "some key",
			"roles": [{"name":"role"}]
		}]`)
		assert.EqualError(t, err, `invalid role name: "role" for service account "nais-service-account"`)
		assert.Len(t, serviceAccounts, 0)
	})

	t.Run("create multiple service accounts and delete old one", func(t *testing.T) {
		ctx := context.Background()
		txCtx := context.Background()
		database := db.NewMockDatabase(t)
		dbtx := db.NewMockDatabase(t)

		sa1 := serviceAccountWithName("nais-service-account-1")
		sa2 := serviceAccountWithName("nais-service-account-2")
		sa3 := serviceAccountWithName("nais-service-account-3")

		database.
			On("Transaction", ctx, mock.AnythingOfType("db.DatabaseTransactionFunc")).
			Run(func(args mock.Arguments) {
				fn := args.Get(1).(db.DatabaseTransactionFunc)
				fn(txCtx, dbtx)
			}).
			Return(nil).
			Once()

		// First service account
		dbtx.
			On("GetServiceAccountByName", txCtx, sa1.Name).
			Return(nil, errors.New("service account not found")).
			Once()
		dbtx.
			On("CreateServiceAccount", txCtx, "nais-service-account-1").
			Return(sa1, nil).
			Once()
		dbtx.
			On("RemoveApiKeysFromServiceAccount", txCtx, sa1.ID).
			Return(nil).
			Once()
		dbtx.
			On("GetServiceAccountRoles", txCtx, sa1.ID).
			Return(nil, nil).
			Once()
		dbtx.
			On("AssignGlobalRoleToServiceAccount", txCtx, sa1.ID, sqlc.RoleNameTeamcreator).
			Return(nil).
			Once()
		dbtx.
			On("AssignGlobalRoleToServiceAccount", txCtx, sa1.ID, sqlc.RoleNameTeamviewer).
			Return(nil).
			Once()
		dbtx.
			On("CreateAPIKey", txCtx, "key-1", sa1.ID).
			Return(nil).
			Once()

		// Second service account, already has the role requested
		dbtx.
			On("GetServiceAccountByName", txCtx, sa2.Name).
			Return(sa2, nil).
			Once()
		dbtx.
			On("RemoveApiKeysFromServiceAccount", txCtx, sa2.ID).
			Return(nil).
			Once()
		dbtx.
			On("GetServiceAccountRoles", txCtx, sa2.ID).
			Return([]*db.Role{{RoleName: sqlc.RoleNameAdmin}}, nil).
			Once()
		dbtx.
			On("CreateAPIKey", txCtx, "key-2", sa2.ID).
			Return(nil).
			Once()

		// Delete old service account
		dbtx.
			On("GetServiceAccounts", txCtx).
			Return([]*db.ServiceAccount{sa1, sa2, sa3}, nil).
			Once()
		dbtx.
			On("DeleteServiceAccount", txCtx, sa3.ID).
			Return(nil).
			Once()

		err := serviceAccounts.Decode(`[{
			"name": "nais-service-account-1",
			"apiKey": "key-1",
			"roles": [{"name":"Team creator"}, {"name":"Team viewer"}]
		}, {
			"name": "nais-service-account-2",
			"apiKey": "key-2",
			"roles": [{"name":"Admin"}]
		}]`)
		assert.NoError(t, err)
		assert.Len(t, serviceAccounts, 2)

		err = fixtures.SetupStaticServiceAccounts(ctx, database, serviceAccounts)
		assert.NoError(t, err)
	})
}

func serviceAccountWithName(name string) *db.ServiceAccount {
	return &db.ServiceAccount{
		ServiceAccount: &sqlc.ServiceAccount{
			ID:   uuid.New(),
			Name: name,
		},
	}
}
