-- Port-forward RBAC grants were removed with the legacy Postgres CRD.
Test.gql("Legacy Postgres grant mutation is no longer available", function(t)
	t.query [[mutation { grantPostgresAccess(input: {
        clusterName: "legacy", teamSlug: "someteamname", environmentName: "dev",
        grantee: "someone@example.com", duration: "1h"
    }) { error } }]]
	t.check { errors = { { message = Contains("grantPostgresAccess") } }, data = Null }
end)
