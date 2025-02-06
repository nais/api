package authz

/*
type Authorization string

const (
	AuthorizationActivityLogsRead      Authorization = "activity_logs:read"
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
	AuthorizationRepositoriesCreate    Authorization = "repositories:create"
	AuthorizationRepositoriesDelete    Authorization = "repositories:delete"
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

var roles = map[authzsql.RoleName][]Authorization{
	authzsql.RoleNameAdmin: {
		// Admins have all authorizations
	},
	authzsql.RoleNameTeamcreator: {
		AuthorizationTeamsCreate,
	},
	authzsql.RoleNameServiceaccountcreator: {
		AuthorizationServiceAccountsCreate,
	},
	authzsql.RoleNameServiceaccountowner: {
		AuthorizationServiceAccountsCreate,
		AuthorizationServiceAccountsDelete,
		AuthorizationServiceAccountsRead,
		AuthorizationServiceAccountsUpdate,
	},
	authzsql.RoleNameTeammember: {
		AuthorizationActivityLogsRead,
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
		AuthorizationRepositoriesCreate,
		AuthorizationRepositoriesDelete,
		AuthorizationServiceAccountsCreate,
		AuthorizationServiceAccountsDelete,
		AuthorizationServiceAccountsRead,
		AuthorizationServiceAccountsUpdate,
	},
	authzsql.RoleNameTeamowner: {
		AuthorizationActivityLogsRead,
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
		AuthorizationRepositoriesCreate,
		AuthorizationRepositoriesDelete,
		AuthorizationServiceAccountsCreate,
		AuthorizationServiceAccountsDelete,
		AuthorizationServiceAccountsRead,
		AuthorizationServiceAccountsUpdate,
	},
	authzsql.RoleNameTeamviewer: {
		AuthorizationActivityLogsRead,
		AuthorizationTeamsList,
		AuthorizationTeamsRead,
	},
	authzsql.RoleNameUseradmin: {
		AuthorizationUsersList,
	},
	authzsql.RoleNameUserviewer: {
		AuthorizationUsersList,
	},
	authzsql.RoleNameSynchronizer: {
		AuthorizationTeamsSynchronize,
		AuthorizationUsersyncSynchronize,
	},
	authzsql.RoleNameDeploykeyviewer: {
		AuthorizationDeployKeyRead,
	},
}


*/
