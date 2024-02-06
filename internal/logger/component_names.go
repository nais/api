package logger

type ComponentName string

const (
	ComponentNameAuthn      ComponentName = "authn"
	ComponentNameConsole    ComponentName = "console"
	ComponentNameGraphqlApi ComponentName = "graphql-api"
	ComponentNameUsersync   ComponentName = "usersync"
)
