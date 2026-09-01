package runner

import (
	"github.com/nais/api/internal/thirdparty/aiven"
	"github.com/nais/tester/lua/spec"
	lua "github.com/yuin/gopher-lua"
)

var _ spec.Runner = &Aiven{}

// Aiven exposes the Aiven fake to Lua so each test can declare the version Aiven
// reports for the services it exercises. Without it every test would have to agree on
// one hardcoded version, which no single value can satisfy once the enum has more than
// one rung.
type Aiven struct {
	client *aiven.FakeAivenClient
}

func NewAivenRunner(client *aiven.FakeAivenClient) *Aiven {
	return &Aiven{client: client}
}

func (a *Aiven) Name() string {
	return "aiven"
}

func (a *Aiven) Functions() []*spec.Function {
	return nil
}

func (a *Aiven) HelperFunctions() []*spec.Function {
	return []*spec.Function{
		{
			Name: "setAivenVersion",
			Args: []spec.Argument{
				{Name: "serviceName", Type: []spec.ArgumentType{spec.ArgumentTypeString}, Doc: `Fully qualified Aiven service name, e.g. "valkey-myteam-versioned"`},
				{Name: "version", Type: []spec.ArgumentType{spec.ArgumentTypeString}, Doc: `Version Aiven reports as running, e.g. "8.1.4"`},
			},
			Doc:  "Set the version Aiven reports as running for a service",
			Func: a.setAivenVersion,
		},
	}
}

func (a *Aiven) setAivenVersion(L *lua.LState) int {
	a.client.SetServiceVersion(L.CheckString(1), L.CheckString(2))
	return 0
}
