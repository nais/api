local user = User.new("postgres-delete-user", "postgres-delete-user@usersen.com")
local team = Team.new("pg-delete-team", "Testing PostgresInstance deletion", "#postgres-delete")
team:addMember(user)
Helper.readK8sResources("k8s_resources/postgres_delete")

Test.gql("Active PostgresInstance cannot be marked for deletion", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[mutation { deletePostgres(input: {
		name: "orders-new", environmentName: "dev", teamSlug: "pg-delete-team"
	}) { postgresDeleted } }]]
	t.check { errors = { { path = { "deletePostgres" }, message = Contains("is active and cannot be deleted") } }, data = Null }
end)

Test.gql("Inactive PostgresInstance can be deleted", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[mutation { deletePostgres(input: {
		name: "orders-old", environmentName: "dev", teamSlug: "pg-delete-team"
	}) { postgresDeleted } }]]
	t.check { data = { deletePostgres = { postgresDeleted = true } } }
end)
