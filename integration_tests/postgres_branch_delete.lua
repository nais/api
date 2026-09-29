local user = User.new("postgres-delete-user", "postgres-delete-user@usersen.com")
local team = Team.new("pg-delete-team", "Testing PostgresBranch deletion", "#postgres-delete")
team:addMember(user)
Helper.readK8sResources("k8s_resources/postgres_branch_delete")

Test.gql("Active PostgresBranch cannot be marked for deletion", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[mutation { deletePostgresBranch(input: {
		name: "orders-new", environmentName: "dev", teamSlug: "pg-delete-team"
	}) { postgresBranchDeleted } }]]
	t.check { errors = { { locations = NotNull(), path = { "deletePostgresBranch" }, message = Contains("is active and cannot be deleted") } }, data = Null }
end)

Test.gql("Inactive PostgresBranch can be deleted", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[mutation { deletePostgresBranch(input: {
		name: "orders-old", environmentName: "dev", teamSlug: "pg-delete-team"
	}) { postgresBranchDeleted } }]]
	t.check { data = { deletePostgresBranch = { postgresBranchDeleted = true } } }
end)
