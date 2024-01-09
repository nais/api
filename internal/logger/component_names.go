package logger

import sqlc "github.com/nais/api/internal/database/gensql"

type ComponentName string

const (
	ComponentNameAuthn      ComponentName = "authn"
	ComponentNameConsole    ComponentName = "console"
	ComponentNameGraphqlApi ComponentName = "graphql-api"
	ComponentNameUsersync   ComponentName = "usersync"

	ComponentNameAzureGroup           = ComponentName(sqlc.ReconcilerNameAzureGroup)
	ComponentNameGithubTeam           = ComponentName(sqlc.ReconcilerNameGithubTeam)
	ComponentNameGoogleGcpGar         = ComponentName(sqlc.ReconcilerNameGoogleGcpGar)
	ComponentNameGoogleGcpProject     = ComponentName(sqlc.ReconcilerNameGoogleGcpProject)
	ComponentNameGoogleWorkspaceAdmin = ComponentName(sqlc.ReconcilerNameGoogleWorkspaceAdmin)
	ComponentNameNaisDeploy           = ComponentName(sqlc.ReconcilerNameNaisDeploy)
	ComponentNameNaisDependencytrack  = ComponentName(sqlc.ReconcilerNameNaisDependencytrack)
	ComponentNameNaisNamespace        = ComponentName(sqlc.ReconcilerNameNaisNamespace)
)
