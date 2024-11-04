package role

import (
	"fmt"

	"github.com/nais/api/internal/v1/role/rolesql"
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

var roles = map[rolesql.RoleName][]Authorization{
	rolesql.RoleNameAdmin: {
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
	rolesql.RoleNameServiceaccountcreator: {
		AuthorizationServiceAccountsCreate,
	},
	rolesql.RoleNameServiceaccountowner: {
		AuthorizationServiceAccountsDelete,
		AuthorizationServiceAccountsRead,
		AuthorizationServiceAccountsUpdate,
	},
	rolesql.RoleNameTeamcreator: {
		AuthorizationTeamsCreate,
	},
	rolesql.RoleNameTeammember: {
		AuthorizationAuditLogsRead,
		AuthorizationTeamsRead,
		AuthorizationTeamsMetadataUpdate,
		AuthorizationDeployKeyView,
		AuthorizationTeamsSynchronize,
	},
	rolesql.RoleNameTeamowner: {
		AuthorizationAuditLogsRead,
		AuthorizationTeamsDelete,
		AuthorizationTeamsRead,
		AuthorizationTeamsMetadataUpdate,
		AuthorizationTeamsSynchronize,
		AuthorizationTeamsMembersAdmin,
		AuthorizationDeployKeyView,
	},
	rolesql.RoleNameTeamviewer: {
		AuthorizationAuditLogsRead,
		AuthorizationTeamsList,
		AuthorizationTeamsRead,
	},
	rolesql.RoleNameUseradmin: {
		AuthorizationUsersList,
	},
	rolesql.RoleNameUserviewer: {
		AuthorizationUsersList,
	},
	rolesql.RoleNameSynchronizer: {
		AuthorizationTeamsSynchronize,
		AuthorizationUsersyncSynchronize,
	},
	rolesql.RoleNameDeploykeyviewer: {
		AuthorizationDeployKeyView,
	},
}

func Authorizations(roleName rolesql.RoleName) ([]Authorization, error) {
	authorizations, exists := roles[roleName]
	if !exists {
		return nil, fmt.Errorf("unknown role: %q", roleName)
	}

	return authorizations, nil
}
