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
	AuthorizationTeamsList             Authorization = "teams:list"
	AuthorizationTeamsRead             Authorization = "teams:read"
	AuthorizationTeamsMembersAdmin     Authorization = "teams:members:admin"
	AuthorizationSecretsList           Authorization = "teams:secrets:list"
	AuthorizationUsersList             Authorization = "users:list"
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
