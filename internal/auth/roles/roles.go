package roles

import (
	"fmt"

	"github.com/nais/api/internal/database/gensql"
)

type Authorization string

const (
	AuthorizationAuditLogsRead         Authorization = "audit_logs:read"
	AuthorizationServiceAccountsCreate Authorization = "service_accounts:create"
	AuthorizationServiceAccountsDelete Authorization = "service_accounts:delete"
	AuthorizationServiceAccountsList   Authorization = "service_accounts:list"
	AuthorizationServiceAccountsRead   Authorization = "service_accounts:read"
	AuthorizationServiceAccountsUpdate Authorization = "service_accounts:update"
	AuthorizationSystemStatesDelete    Authorization = "system_states:delete"
	AuthorizationSystemStatesRead      Authorization = "system_states:read"
	AuthorizationSystemStatesUpdate    Authorization = "system_states:update"
	AuthorizationTeamsCreate           Authorization = "teams:create"
	AuthorizationTeamsDelete           Authorization = "teams:delete"
	AuthorizationTeamsList             Authorization = "teams:list"
	AuthorizationTeamsRead             Authorization = "teams:read"
	AuthorizationTeamsMetadataUpdate   Authorization = "teams:metadata:update"
	AuthorizationTeamsMembersAdmin     Authorization = "teams:members:admin"
	AuthorizationUsersList             Authorization = "users:list"
	AuthorizationTeamsSynchronize      Authorization = "teams:synchronize"
	AuthorizationUsersyncSynchronize   Authorization = "usersync:synchronize"
	AuthorizationDeployKeyView         Authorization = "deploy_key:view"
)

var roles = map[gensql.RoleName][]Authorization{
	gensql.RoleNameAdmin: {
		AuthorizationAuditLogsRead,
		AuthorizationServiceAccountsCreate,
		AuthorizationServiceAccountsDelete,
		AuthorizationServiceAccountsList,
		AuthorizationServiceAccountsRead,
		AuthorizationServiceAccountsUpdate,
		AuthorizationSystemStatesDelete,
		AuthorizationSystemStatesRead,
		AuthorizationSystemStatesUpdate,
		AuthorizationTeamsCreate,
		AuthorizationTeamsDelete,
		AuthorizationTeamsList,
		AuthorizationTeamsRead,
		AuthorizationTeamsMembersAdmin,
		AuthorizationUsersList,
		AuthorizationTeamsSynchronize,
		AuthorizationUsersyncSynchronize,
		AuthorizationDeployKeyView,
	},
	gensql.RoleNameServiceaccountcreator: {
		AuthorizationServiceAccountsCreate,
	},
	gensql.RoleNameServiceaccountowner: {
		AuthorizationServiceAccountsDelete,
		AuthorizationServiceAccountsRead,
		AuthorizationServiceAccountsUpdate,
	},
	gensql.RoleNameTeamcreator: {
		AuthorizationTeamsCreate,
	},
	gensql.RoleNameTeammember: {
		AuthorizationAuditLogsRead,
		AuthorizationTeamsRead,
		AuthorizationTeamsMetadataUpdate,
		AuthorizationDeployKeyView,
		AuthorizationTeamsSynchronize,
	},
	gensql.RoleNameTeamowner: {
		AuthorizationAuditLogsRead,
		AuthorizationTeamsDelete,
		AuthorizationTeamsRead,
		AuthorizationTeamsMetadataUpdate,
		AuthorizationTeamsSynchronize,
		AuthorizationTeamsMembersAdmin,
		AuthorizationDeployKeyView,
	},
	gensql.RoleNameTeamviewer: {
		AuthorizationAuditLogsRead,
		AuthorizationTeamsList,
		AuthorizationTeamsRead,
	},
	gensql.RoleNameUseradmin: {
		AuthorizationUsersList,
	},
	gensql.RoleNameUserviewer: {
		AuthorizationUsersList,
	},
	gensql.RoleNameSynchronizer: {
		AuthorizationTeamsSynchronize,
		AuthorizationUsersyncSynchronize,
	},
	gensql.RoleNameDeploykeyviewer: {
		AuthorizationDeployKeyView,
	},
}

func Authorizations(roleName gensql.RoleName) ([]Authorization, error) {
	authorizations, exists := roles[roleName]
	if !exists {
		return nil, fmt.Errorf("unknown role: %q", roleName)
	}

	return authorizations, nil
}
