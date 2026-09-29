-- The old per-instance audit flag and Cloud SQL Logs URL are not part of
-- nais.io/v1 PostgresInstance. Verify logical configuration instead.
Helper.readK8sResources("k8s_resources/postgres_audit_log")
local user = User.new("authenticated", "postgres-audit-user@example.com", "postgres-audit-user-id")
local team = Team.new("audit-postgres-team", "Testing logical Postgres", "#audit-postgres")
team:addMember(user)

Test.gql("Logical Postgres settings are not fabricated on physical instances", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug:"audit-postgres-team") { environment(name:"dev-gcp") {
        postgresInstance(name:"audit-enabled") { name state postgres { name majorVersion } }
    } } }]]
	t.check { data = { team = { environment = { postgresInstance = {
		name = "audit-enabled", state = "AVAILABLE", postgres = { name = "audit-enabled", majorVersion = "16" },
	} } } } }
end)
