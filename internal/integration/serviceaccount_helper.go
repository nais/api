//go:build integration_test

package integration

import (
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/auth/authz/authzsql"
	"github.com/nais/api/internal/serviceaccount"
	"github.com/nais/api/internal/serviceaccount/serviceaccountsql"
	"github.com/nais/api/internal/slug"
	"github.com/nais/tester/lua/spec"
	lua "github.com/yuin/gopher-lua"
)

const luaServiceAccountTypeName = "ServiceAccount"

type ServiceAccount struct {
	ID       uuid.UUID
	Name     string
	TeamSlug *slug.Slug
	// The plaintext token, only available after creation
	Token string
}

func serviceAccountMetatable() *spec.Typemetatable {
	return &spec.Typemetatable{
		Name: luaServiceAccountTypeName,
		Init: &spec.Function{
			Doc: "Create a new service account with a token",
			Args: []spec.Argument{
				{
					Name: "name",
					Type: []spec.ArgumentType{spec.ArgumentTypeString},
					Doc:  "The name of the service account",
				},
				{
					Name: "teamSlug?",
					Type: []spec.ArgumentType{spec.ArgumentTypeString},
					Doc:  "The team slug to scope the service account to (optional, global if omitted)",
				},
			},
			Func: createServiceAccount,
		},
		GetSet: []spec.TypemetatableGetSet{
			{
				Name:       "id",
				Doc:        "The id of the service account",
				GetReturns: []spec.ArgumentType{spec.ArgumentTypeString},
				Func:       serviceAccountGetID,
			},
			{
				Name:       "name",
				Doc:        "The name of the service account",
				GetReturns: []spec.ArgumentType{spec.ArgumentTypeString},
				Func:       serviceAccountGetName,
			},
			{
				Name:       "token",
				Doc:        "The bearer token for authenticating as this service account",
				GetReturns: []spec.ArgumentType{spec.ArgumentTypeString},
				Func:       serviceAccountGetToken,
			},
		},
		Methods: []spec.Function{
			{
				Name: "assignRole",
				Doc:  "Assign a role to this service account",
				Args: []spec.Argument{
					{
						Name: "roleName",
						Type: []spec.ArgumentType{spec.ArgumentTypeString},
						Doc:  "The name of the role to assign",
					},
				},
				Func: serviceAccountAssignRole,
			},
		},
	}
}

func createServiceAccount(L *lua.LState) int {
	pool := L.Context().Value(databaseKey).(*pgxpool.Pool)
	saDB := serviceaccountsql.New(pool)

	name := L.CheckString(1)

	var teamSlug *slug.Slug
	if ts := L.OptString(2, ""); ts != "" {
		s := slug.Slug(ts)
		teamSlug = &s
	}

	sa, err := saDB.Create(L.Context(), serviceaccountsql.CreateParams{
		Name:        name,
		Description: "Integration test service account",
		TeamSlug:    teamSlug,
	})
	if err != nil {
		L.RaiseError("failed to create service account: %s", err)
		return 0
	}

	// Generate a token and store its hash
	plaintext, err := serviceaccount.GenerateToken()
	if err != nil {
		L.RaiseError("failed to generate token: %s", err)
		return 0
	}

	hashed, err := serviceaccount.HashToken(plaintext)
	if err != nil {
		L.RaiseError("failed to hash token: %s", err)
		return 0
	}

	_, err = saDB.CreateToken(L.Context(), serviceaccountsql.CreateTokenParams{
		Name:             "test-token",
		Description:      "Integration test token",
		Token:            hashed,
		ServiceAccountID: sa.ID,
		ExpiresAt:        pgtype.Date{}, // no expiry
	})
	if err != nil {
		L.RaiseError("failed to create token: %s", err)
		return 0
	}

	ret := &ServiceAccount{
		ID:       sa.ID,
		Name:     sa.Name,
		TeamSlug: teamSlug,
		Token:    plaintext,
	}

	ud := L.NewUserData()
	ud.Value = ret
	L.SetMetatable(ud, L.GetTypeMetatable(luaServiceAccountTypeName))
	L.Push(ud)
	return 1
}

func serviceAccountGetID(L *lua.LState) int {
	sa := checkServiceAccount(L)
	L.Push(lua.LString(sa.ID.String()))
	return 1
}

func serviceAccountGetName(L *lua.LState) int {
	sa := checkServiceAccount(L)
	L.Push(lua.LString(sa.Name))
	return 1
}

func serviceAccountGetToken(L *lua.LState) int {
	sa := checkServiceAccount(L)
	L.Push(lua.LString(sa.Token))
	return 1
}

func serviceAccountAssignRole(L *lua.LState) int {
	sa := checkServiceAccount(L)
	roleName := L.CheckString(2)

	pool := L.Context().Value(databaseKey).(*pgxpool.Pool)
	authzDB := authzsql.New(pool)

	err := authzDB.AssignRoleToServiceAccount(L.Context(), authzsql.AssignRoleToServiceAccountParams{
		ServiceAccountID: sa.ID,
		RoleName:         roleName,
	})
	if err != nil {
		L.RaiseError("failed to assign role %q to service account: %s", roleName, err)
		return 0
	}

	return 0
}

func checkServiceAccount(L *lua.LState) *ServiceAccount {
	ud := L.CheckUserData(1)
	if v, ok := ud.Value.(*ServiceAccount); ok {
		return v
	}
	L.ArgError(1, "ServiceAccount expected")
	return nil
}
