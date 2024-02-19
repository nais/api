package logger

type ComponentName string

const (
	ComponentNameAuthn      ComponentName = "authn"
	ComponentNameGraphqlApi ComponentName = "graphql-api"
	ComponentNameUsersync   ComponentName = "usersync"
)
