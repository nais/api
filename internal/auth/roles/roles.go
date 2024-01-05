package roles

import (
	"fmt"

	"github.com/nais/api/internal/sqlc"
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
	AuthorizationTeamsUpdate           Authorization = "teams:update"
	AuthorizationUsersList             Authorization = "users:list"
	AuthorizationUsersUpdate           Authorization = "users:update"
	AuthorizationTeamsSynchronize      Authorization = "teams:synchronize"
	AuthorizationUsersyncSynchronize   Authorization = "usersync:synchronize"
	AuthorizationDeployKeyView         Authorization = "deploy_key:view"
)

var roles = map[sqlc.RoleName][]Authorization{
	sqlc.RoleNameAdmin: {
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
		AuthorizationTeamsUpdate,
		AuthorizationUsersList,
		AuthorizationUsersUpdate,
		AuthorizationTeamsSynchronize,
		AuthorizationUsersyncSynchronize,
		AuthorizationDeployKeyView,
	},
	sqlc.RoleNameServiceaccountcreator: {
		AuthorizationServiceAccountsCreate,
	},
	sqlc.RoleNameServiceaccountowner: {
		AuthorizationServiceAccountsDelete,
		AuthorizationServiceAccountsRead,
		AuthorizationServiceAccountsUpdate,
	},
	sqlc.RoleNameTeamcreator: {
		AuthorizationTeamsCreate,
	},
	sqlc.RoleNameTeammember: {
		AuthorizationAuditLogsRead,
		AuthorizationTeamsRead,
		AuthorizationDeployKeyView,
		AuthorizationTeamsSynchronize,
	},
	sqlc.RoleNameTeamowner: {
		AuthorizationAuditLogsRead,
		AuthorizationTeamsDelete,
		AuthorizationTeamsRead,
		AuthorizationTeamsUpdate,
		AuthorizationTeamsSynchronize,
		AuthorizationDeployKeyView,
	},
	sqlc.RoleNameTeamviewer: {
		AuthorizationAuditLogsRead,
		AuthorizationTeamsList,
		AuthorizationTeamsRead,
	},
	sqlc.RoleNameUseradmin: {
		AuthorizationUsersList,
		AuthorizationUsersUpdate,
	},
	sqlc.RoleNameUserviewer: {
		AuthorizationUsersList,
	},
	sqlc.RoleNameSynchronizer: {
		AuthorizationTeamsSynchronize,
		AuthorizationUsersyncSynchronize,
	},
	sqlc.RoleNameDeploykeyviewer: {
		AuthorizationDeployKeyView,
	},
}

func Authorizations(roleName sqlc.RoleName) ([]Authorization, error) {
	authorizations, exists := roles[roleName]
	if !exists {
		return nil, fmt.Errorf("unknown role: %q", roleName)
	}

	return authorizations, nil
}
