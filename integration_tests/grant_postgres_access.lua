-- The old port-forward grant mutation must not reappear in the public schema.
local user = User.new("postgres-grant-check", "postgres-grant-check@usersen.com")

Test.gql("Legacy Postgres grant mutation is no longer available", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[mutation { grantPostgresAccess(input: {
        clusterName: "legacy", teamSlug: "someteamname", environmentName: "dev",
        grantee: "someone@example.com", duration: "1h"
    }) { error } }]]
	t.check { errors = { { message = Contains("Cannot query field \"grantPostgresAccess\""), locations = NotNull() } }, data = Null }
end)
