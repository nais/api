//go:build integration_test

package integration

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/team/teamsql"
	"github.com/nais/tester/lua/spec"
	lua "github.com/yuin/gopher-lua"
	"k8s.io/utils/ptr"
)

const luaTeamTypeName = "Team"

type Team struct {
	Slug         slug.Slug
	Purpose      string
	SlackChannel string
}

func teamMetatable() *spec.Typemetatable {
	return &spec.Typemetatable{
		Name: luaTeamTypeName,
		Init: &spec.Function{
			Doc: "Create a new team",
			Args: []spec.Argument{
				{
					Name: "slug",
					Type: []spec.ArgumentType{spec.ArgumentTypeString},
					Doc:  "The slug of the team to create",
				},
				{
					Name: "purpose",
					Type: []spec.ArgumentType{spec.ArgumentTypeString},
					Doc:  "The purpose of the team to create",
				},
				{
					Name: "slackChannel",
					Type: []spec.ArgumentType{spec.ArgumentTypeString},
					Doc:  "The slack channel of the team to create",
				},
			},
			Func: createTeam,
		},
		GetSet: []spec.TypemetatableGetSet{
			{
				Name:       "slug",
				Doc:        "The slug of the team",
				GetReturns: []spec.ArgumentType{spec.ArgumentTypeString},
				Func:       teamGetSlug,
			},
			{
				Name:         "purpose",
				Doc:          "The purpose of the team",
				GetReturns:   []spec.ArgumentType{spec.ArgumentTypeString},
				SetArguments: []spec.Argument{{Name: "purpose", Type: []spec.ArgumentType{spec.ArgumentTypeString}}},
				Func:         teamGetSetPurpose,
			},
		},
		Methods: []spec.Function{
			{
				Name: "addMember",
				Doc:  "Add a member to the team",
				Args: []spec.Argument{
					{Name: "...", Type: []spec.ArgumentType{spec.ArgumentTypeMetatable("User")}, Doc: "The user IDs to add to the team"},
				},
				Func: teamAddMember,
			},
			{
				Name: "addOwner",
				Doc:  "Add a owner to the team",
				Args: []spec.Argument{
					{Name: "...", Type: []spec.ArgumentType{spec.ArgumentTypeMetatable("User")}, Doc: "The user IDs to add to the team"},
				},
				Func: teamAddOwner,
			},
		},
	}
}

func createTeam(L *lua.LState) int {
	pool := L.Context().Value(databaseKey).(*pgxpool.Pool)
	db := teamsql.New(pool)

	team, err := db.Create(L.Context(), teamsql.CreateParams{
		Slug:         slug.Slug(L.CheckString(1)),
		Purpose:      L.CheckString(2),
		SlackChannel: L.CheckString(3),
	})
	if err != nil {
		L.RaiseError("failed to create team: %s", err)
		return 0
	}

	ret := &Team{
		Slug:         team.Slug,
		Purpose:      team.Purpose,
		SlackChannel: team.SlackChannel,
	}
	ud := L.NewUserData()
	ud.Value = ret
	L.SetMetatable(ud, L.GetTypeMetatable(luaTeamTypeName))
	L.Push(ud)
	return 1
}

func teamGetSlug(L *lua.LState) int {
	t := checkTeam(L)
	if L.GetTop() == 2 {
		L.ArgError(2, "cannot set slug")
	}
	L.Push(lua.LString(t.Slug))
	return 1
}

func teamGetSetPurpose(L *lua.LState) int {
	t := checkTeam(L)
	if L.GetTop() == 2 {
		db := teamsql.New(L.Context().Value(databaseKey).(*pgxpool.Pool))
		team, err := db.Update(L.Context(), teamsql.UpdateParams{
			Slug:    t.Slug,
			Purpose: ptr.To(L.CheckString(2)),
		})
		if err != nil {
			L.RaiseError("failed to set team purpose: %s", err)
			return 0
		}

		t.Purpose = team.Purpose
		return 0
	}
	L.Push(lua.LString(t.Purpose))
	return 1
}

func addTeamRole(L *lua.LState, role string) int {
	t := checkTeam(L)
	db := teamsql.New(L.Context().Value(databaseKey).(*pgxpool.Pool))

	users := []*User{}
	for i := 2; i <= L.GetTop(); i++ {
		users = append(users, checkUserL(L, i))
	}

	for _, u := range users {
		err := db.AddMember(L.Context(), teamsql.AddMemberParams{
			UserID:   u.ID,
			TeamSlug: slug.Slug(t.Slug),
			RoleName: role,
		})
		if err != nil {
			L.RaiseError("failed to add members to team: %s", err)
			return 0
		}
	}

	return 0
}

func teamAddMember(L *lua.LState) int {
	return addTeamRole(L, "Team member")
}

func teamAddOwner(L *lua.LState) int {
	return addTeamRole(L, "Team owner")
}

func checkTeam(L *lua.LState) *Team {
	ud := L.CheckUserData(1)
	if v, ok := ud.Value.(*Team); ok {
		return v
	}
	L.ArgError(1, "Team expected")
	return nil
}
