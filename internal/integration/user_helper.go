//go:build integration_test

package integration

import (
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/usersync/usersyncer"
	"github.com/nais/api/internal/usersync/usersyncsql"
	"github.com/nais/tester/lua/spec"
	lua "github.com/yuin/gopher-lua"
)

const luaUserTypeName = "User"

type User struct {
	ID         uuid.UUID
	Name       string
	Email      string
	ExternalID string
	Admin      bool
}

func userMetatable() *spec.Typemetatable {
	return &spec.Typemetatable{
		Name: luaUserTypeName,
		Init: &spec.Function{
			Doc: "Create a new user",
			Args: []spec.Argument{
				{
					Name: "name?",
					Type: []spec.ArgumentType{spec.ArgumentTypeString},
					Doc:  "The name of the user to create",
				},
				{
					Name: "email?",
					Type: []spec.ArgumentType{spec.ArgumentTypeString},
					Doc:  "The email of the user to create",
				},
				{
					Name: "externalID?",
					Type: []spec.ArgumentType{spec.ArgumentTypeString},
					Doc:  "The externalID of the user to create",
				},
			},
			Func: createUser,
		},
		GetSet: []spec.TypemetatableGetSet{
			{
				Name:       "id",
				Doc:        "The id of the user",
				GetReturns: []spec.ArgumentType{spec.ArgumentTypeString},
				Func:       userGetID,
			},
			{
				Name:         "name",
				Doc:          "The name of the user",
				GetReturns:   []spec.ArgumentType{spec.ArgumentTypeString},
				SetArguments: []spec.Argument{{Name: "name", Type: []spec.ArgumentType{spec.ArgumentTypeString}}},
				Func:         userGetSetName,
			},
			{
				Name:         "email",
				Doc:          "The email of the user",
				GetReturns:   []spec.ArgumentType{spec.ArgumentTypeString},
				SetArguments: []spec.Argument{{Name: "email", Type: []spec.ArgumentType{spec.ArgumentTypeString}}},
				Func:         userGetSetEmail,
			},
			{
				Name:         "externalID",
				Doc:          "The externalID of the user",
				GetReturns:   []spec.ArgumentType{spec.ArgumentTypeString},
				SetArguments: []spec.Argument{{Name: "externalID", Type: []spec.ArgumentType{spec.ArgumentTypeString}}},
				Func:         userGetSetExternalID,
			},
			{
				Name:         "admin",
				Doc:          "The admin of the user",
				GetReturns:   []spec.ArgumentType{spec.ArgumentTypeBoolean},
				SetArguments: []spec.Argument{{Name: "admin", Type: []spec.ArgumentType{spec.ArgumentTypeBoolean}}},
				Func:         userGetSetAdmin,
			},
		},
	}
}

type userIndexer struct {
	lock sync.Mutex
	i    int
}

func (u *userIndexer) Next() int {
	u.lock.Lock()
	defer u.lock.Unlock()
	u.i++
	return u.i
}

var userIndex = &userIndexer{}

func createUser(L *lua.LState) int {
	pool := L.Context().Value(databaseKey).(*pgxpool.Pool)
	db := usersyncsql.New(pool)

	name := L.OptString(1, "")
	email := L.OptString(2, "")
	externalID := L.OptString(3, "")

	if name == "" {
		name = fmt.Sprintf("name-%v", userIndex.Next())
	}
	if email == "" {
		email = fmt.Sprintf("email-%v@example.com", userIndex.Next())
	}
	if externalID == "" {
		externalID = uuid.NewString()
	}

	user, err := db.Create(L.Context(), usersyncsql.CreateParams{
		Name:       name,
		Email:      email,
		ExternalID: externalID,
	})
	if err != nil {
		L.RaiseError("failed to create user: %s", err)
	}

	if err := usersyncer.AssignDefaultPermissionsToUser(L.Context(), db, user.ID); err != nil {
		L.RaiseError("failed to assign default permissions to user: %s", err)
	}

	ret := &User{
		ID:         user.ID,
		Name:       user.Name,
		Email:      user.Email,
		ExternalID: user.ExternalID,
		Admin:      user.Admin,
	}
	ud := L.NewUserData()
	ud.Value = ret
	L.SetMetatable(ud, L.GetTypeMetatable(luaUserTypeName))
	L.Push(ud)
	return 1
}

func userGetID(L *lua.LState) int {
	t := checkUser(L)
	if L.GetTop() == 2 {
		L.ArgError(2, "cannot set id")
	}
	L.Push(lua.LString(t.ID.String()))
	return 1
}

func userUpdate(L *lua.LState, u *User) {
	db := usersyncsql.New(L.Context().Value(databaseKey).(*pgxpool.Pool))
	err := db.Update(L.Context(), usersyncsql.UpdateParams{
		Name:       u.Name,
		Email:      u.Email,
		ExternalID: u.ExternalID,
		ID:         u.ID,
	})
	if err != nil {
		L.RaiseError("failed to update user: %s", err)
	}
}

func userGetSetName(L *lua.LState) int {
	u := checkUser(L)
	if L.GetTop() == 2 {
		u.Name = L.CheckString(2)
		userUpdate(L, u)
		return 0
	}

	L.Push(lua.LString(u.Name))
	return 1
}

func userGetSetEmail(L *lua.LState) int {
	u := checkUser(L)
	if L.GetTop() == 2 {
		u.Email = L.CheckString(2)
		userUpdate(L, u)
	}

	L.Push(lua.LString(u.Email))
	return 1
}

func userGetSetExternalID(L *lua.LState) int {
	u := checkUser(L)
	if L.GetTop() == 2 {
		u.ExternalID = L.CheckString(2)
		userUpdate(L, u)
	}

	L.Push(lua.LString(u.ExternalID))
	return 1
}

func userGetSetAdmin(L *lua.LState) int {
	u := checkUser(L)
	if L.GetTop() == 2 {
		db := usersyncsql.New(L.Context().Value(databaseKey).(*pgxpool.Pool))
		if L.CheckBool(2) {
			if err := db.AssignGlobalAdmin(L.Context(), u.ID); err != nil {
				L.RaiseError("failed to assign admin: %s", err)
			}
			u.Admin = true
		} else {
			if err := db.RevokeGlobalAdmin(L.Context(), u.ID); err != nil {
				L.RaiseError("failed to revoke admin: %s", err)
			}
			u.Admin = false
		}
		return 0
	}

	L.Push(lua.LBool(u.Admin))
	return 1
}

func checkUser(L *lua.LState) *User {
	ud := L.CheckUserData(1)
	if v, ok := ud.Value.(*User); ok {
		return v
	}
	L.ArgError(1, "User expected")
	return nil
}

func checkUserL(L *lua.LState, idx int) *User {
	ud := L.CheckUserData(idx)
	if v, ok := ud.Value.(*User); ok {
		return v
	}
	L.ArgError(1, "User expected")
	return nil
}
