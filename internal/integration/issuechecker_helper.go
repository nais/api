//go:build integration_test

package integration

import (
	"github.com/nais/api/internal/issue/checker"
	"github.com/nais/tester/lua/spec"
	lua "github.com/yuin/gopher-lua"
)

const luaIssueCheckerTypeName = "IssueChecker"

func issueCheckerMetatable() *spec.Typemetatable {
	return &spec.Typemetatable{
		Name: luaIssueCheckerTypeName,
		Init: &spec.Function{
			Doc:  "Initialize the issue checker",
			Args: []spec.Argument{},
			Func: initializeChecker,
		},
		Methods: []spec.Function{
			{
				Name: "runChecks",
				Doc:  "Run issue checks",
				Func: runChecks,
			},
		},
	}
}

func initializeChecker(L *lua.LState) int {
	ud := L.NewUserData()
	ud.Value = struct{}{}
	L.SetMetatable(ud, L.GetTypeMetatable(luaIssueCheckerTypeName))
	L.Push(ud)
	return 1
}

func runChecks(L *lua.LState) int {
	checker := L.Context().Value("issue_checker").(*checker.Checker)
	checker.RunChecksOnce(L.Context())
	return 1
}
