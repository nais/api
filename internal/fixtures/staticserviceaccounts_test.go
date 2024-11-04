package fixtures_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/fixtures"
	"github.com/nais/api/internal/v1/role"
	"github.com/nais/api/internal/v1/role/rolesql"
	"github.com/stretchr/testify/mock"
)

func TestSetupStaticServiceAccounts(t *testing.T) {
	serviceAccounts := make(fixtures.ServiceAccounts, 0)

	t.Run("empty string", func(t *testing.T) {
		err := serviceAccounts.UnmarshalJSON([]byte(""))
		if err != nil {
			t.Errorf("unexpected error: %q", err)
		}

		if len(serviceAccounts) != 0 {
			t.Errorf("expected no service accounts, got %v", len(serviceAccounts))
		}
	})

	t.Run("invalid data", func(t *testing.T) {
		err := serviceAccounts.UnmarshalJSON([]byte(`{"foo":"bar"}`))
		tgt := &json.UnmarshalTypeError{}
		if !errors.As(err, &tgt) {
			t.Errorf("expected error to be of type json.UnmarshalTypeError, got %T", err)
		}
		if len(serviceAccounts) != 0 {
			t.Errorf("expected no service accounts, got %v", len(serviceAccounts))
		}
	})

	t.Run("service account with no roles", func(t *testing.T) {
		err := serviceAccounts.UnmarshalJSON([]byte(`[{
			"name": "nais-service-account",
			"apiKey": "some key",
			"roles": []
		}]`))
		if err.Error() != `service account must have at least one role: "nais-service-account"` {
			t.Errorf("expected error to contain 'service account must have at least one role: \"nais-service-account\"', got %q", err)
		}
		if len(serviceAccounts) != 0 {
			t.Errorf("expected no service accounts, got %v", len(serviceAccounts))
		}
	})

	t.Run("missing API key", func(t *testing.T) {
		err := serviceAccounts.UnmarshalJSON([]byte(`[{
			"name": "nais-service-account",
			"roles": [{"name":"Admin"}]
		}]`))
		if err.Error() != `service account is missing an API key: "nais-service-account"` {
			t.Errorf("expected error to contain 'service account is missing an API key: \"nais-service-account\"', got %q", err)
		}
		if len(serviceAccounts) != 0 {
			t.Errorf("expected no service accounts, got %v", len(serviceAccounts))
		}
	})

	t.Run("service account with invalid name", func(t *testing.T) {
		err := serviceAccounts.UnmarshalJSON([]byte(`[{
			"name": "service-account",
			"apiKey": "some key",
			"roles": [{"name":"Team viewer"}]
		}]`))
		if err.Error() != `service account is missing required "nais-" prefix: "service-account"` {
			t.Errorf("expected error to contain 'service account is missing required \"nais-\" prefix: \"service-account\"', got %q", err)
		}
		if len(serviceAccounts) != 0 {
			t.Errorf("expected no service accounts, got %v", len(serviceAccounts))
		}
	})

	t.Run("service account with invalid role", func(t *testing.T) {
		err := serviceAccounts.UnmarshalJSON([]byte(`[{
			"name": "nais-service-account",
			"apiKey": "some key",
			"roles": [{"name":"role"}]
		}]`))
		if err.Error() != `invalid role name: "role" for service account "nais-service-account"` {
			t.Errorf("expected error to contain 'invalid role name: \"role\" for service account \"nais-service-account\"', got %q", err)
		}
		if len(serviceAccounts) != 0 {
			t.Errorf("expected no service accounts, got %v", len(serviceAccounts))
		}
	})

	t.Run("create multiple service accounts and delete old one", func(t *testing.T) {
		ctx := context.Background()
		txCtx := context.Background()
		db := database.NewMockDatabase(t)
		dbtx := database.NewMockDatabase(t)

		sa1 := serviceAccountWithName("nais-service-account-1")
		sa2 := serviceAccountWithName("nais-service-account-2")
		sa3 := serviceAccountWithName("nais-service-account-3")

		db.EXPECT().
			Transaction(ctx, mock.AnythingOfType("database.DatabaseTransactionFunc")).
			Run(func(ctx context.Context, fn database.DatabaseTransactionFunc) {
				fn(txCtx, dbtx)
			}).
			Return(nil).
			Once()

		// First service account
		dbtx.EXPECT().
			GetServiceAccountByName(txCtx, sa1.Name).
			Return(nil, errors.New("service account not found")).
			Once()
		dbtx.EXPECT().
			CreateServiceAccount(txCtx, "nais-service-account-1").
			Return(sa1, nil).
			Once()
		dbtx.EXPECT().
			RemoveApiKeysFromServiceAccount(txCtx, sa1.ID).
			Return(nil).
			Once()
		dbtx.EXPECT().
			GetServiceAccountRoles(txCtx, sa1.ID).
			Return(nil, nil).
			Once()
		dbtx.EXPECT().
			AssignGlobalRoleToServiceAccount(txCtx, sa1.ID, gensql.RoleNameTeamcreator).
			Return(nil).
			Once()
		dbtx.EXPECT().
			AssignGlobalRoleToServiceAccount(txCtx, sa1.ID, gensql.RoleNameTeamviewer).
			Return(nil).
			Once()
		dbtx.EXPECT().
			CreateAPIKey(txCtx, "key-1", sa1.ID).
			Return(nil).
			Once()

		// Second service account, already has the role requested
		dbtx.EXPECT().
			GetServiceAccountByName(txCtx, sa2.Name).
			Return(sa2, nil).
			Once()
		dbtx.EXPECT().
			RemoveApiKeysFromServiceAccount(txCtx, sa2.ID).
			Return(nil).
			Once()
		dbtx.EXPECT().
			GetServiceAccountRoles(txCtx, sa2.ID).
			Return([]*role.Role{{Name: rolesql.RoleNameAdmin}}, nil).
			Once()
		dbtx.EXPECT().
			CreateAPIKey(txCtx, "key-2", sa2.ID).
			Return(nil).
			Once()

		// Delete old service account
		dbtx.EXPECT().
			GetServiceAccounts(txCtx).
			Return([]*database.ServiceAccount{sa1, sa2, sa3}, nil).
			Once()
		dbtx.EXPECT().
			DeleteServiceAccount(txCtx, sa3.ID).
			Return(nil).
			Once()

		err := serviceAccounts.UnmarshalJSON([]byte(`[{
			"name": "nais-service-account-1",
			"apiKey": "key-1",
			"roles": [{"name":"Team creator"}, {"name":"Team viewer"}]
		}, {
			"name": "nais-service-account-2",
			"apiKey": "key-2",
			"roles": [{"name":"Admin"}]
		}]`))
		if err != nil {
			t.Fatal(err)
		}
		if len(serviceAccounts) != 2 {
			t.Errorf("expected 2 service accounts, got %v", len(serviceAccounts))
		}

		err = fixtures.SetupStaticServiceAccounts(ctx, db, serviceAccounts)
		if err != nil {
			t.Fatal(err)
		}
	})
}

func serviceAccountWithName(name string) *database.ServiceAccount {
	return &database.ServiceAccount{
		ServiceAccount: &gensql.ServiceAccount{
			ID:   uuid.New(),
			Name: name,
		},
	}
}
