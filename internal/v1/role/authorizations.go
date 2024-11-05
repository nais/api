package role

import (
	"github.com/nais/api/internal/v1/role/rolesql"
)

type Authorization string

const (
	AuthorizationAuditLogsRead         Authorization = "audit_logs:read"
	AuthorizationServiceAccountsCreate Authorization = "service_accounts:create"
	AuthorizationServiceAccountsDelete Authorization = "service_accounts:delete"
	AuthorizationServiceAccountsRead   Authorization = "service_accounts:read"
	AuthorizationServiceAccountsUpdate Authorization = "service_accounts:update"
	AuthorizationTeamsCreate           Authorization = "teams:create"
	AuthorizationTeamsDelete           Authorization = "teams:delete"
	AuthorizationTeamsList             Authorization = "teams:list"
	AuthorizationTeamsRead             Authorization = "teams:read"
	AuthorizationTeamsMetadataUpdate   Authorization = "teams:metadata:update"
	AuthorizationTeamsMembersAdmin     Authorization = "teams:members:admin"
	AuthorizationSecretsCreate         Authorization = "teams:secrets:create"
	AuthorizationSecretsDelete         Authorization = "teams:secrets:delete"
	AuthorizationSecretsUpdate         Authorization = "teams:secrets:update"
	AuthorizationSecretsRead           Authorization = "teams:secrets:read"
	AuthorizationSecretsList           Authorization = "teams:secrets:list"
	AuthorizationApplicationsUpdate    Authorization = "applications:update"
	AuthorizationApplicationsDelete    Authorization = "applications:delete"
	AuthorizationJobsUpdate            Authorization = "jobs:update"
	AuthorizationJobsDelete            Authorization = "jobs:delete"
	AuthorizationUsersList             Authorization = "users:list"
	AuthorizationTeamsSynchronize      Authorization = "teams:synchronize"
	AuthorizationUsersyncSynchronize   Authorization = "usersync:synchronize"
	AuthorizationDeployKeyRead         Authorization = "deploy_key:read"
	AuthorizationDeployKeyUpdate       Authorization = "deploy_key:update"
	AuthorizationUnleashCreate         Authorization = "unleash:create"
	AuthorizationUnleashUpdate         Authorization = "unleash:update"
)

var roles = map[rolesql.RoleName][]Authorization{
	rolesql.RoleNameAdmin: {
		// Admins have all authorizations
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
		AuthorizationDeployKeyRead,
		AuthorizationTeamsSynchronize,
		AuthorizationJobsDelete,
		AuthorizationJobsUpdate,
		AuthorizationSecretsCreate,
		AuthorizationSecretsDelete,
		AuthorizationSecretsUpdate,
		AuthorizationSecretsRead,
		AuthorizationSecretsList,
		AuthorizationDeployKeyUpdate,
		AuthorizationUnleashCreate,
		AuthorizationUnleashUpdate,
		AuthorizationApplicationsUpdate,
		AuthorizationApplicationsDelete,
	},
	rolesql.RoleNameTeamowner: {
		AuthorizationAuditLogsRead,
		AuthorizationTeamsDelete,
		AuthorizationTeamsRead,
		AuthorizationTeamsMetadataUpdate,
		AuthorizationTeamsSynchronize,
		AuthorizationTeamsMembersAdmin,
		AuthorizationDeployKeyRead,
		AuthorizationJobsDelete,
		AuthorizationJobsUpdate,
		AuthorizationSecretsCreate,
		AuthorizationSecretsDelete,
		AuthorizationSecretsUpdate,
		AuthorizationSecretsRead,
		AuthorizationSecretsList,
		AuthorizationDeployKeyUpdate,
		AuthorizationUnleashCreate,
		AuthorizationUnleashUpdate,
		AuthorizationApplicationsUpdate,
		AuthorizationApplicationsDelete,
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
		AuthorizationDeployKeyRead,
	},
}
