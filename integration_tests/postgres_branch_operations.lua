local member = User.new("pg-branch-user", "pg-branch-user@usersen.com")
local outsider = User.new("pg-branch-outsider", "pg-branch-outsider@usersen.com")
local team = Team.new("pg-crud-team", "Testing Postgres branch operations", "#postgres-crud")
team:addMember(member)
Helper.readK8sResources("k8s_resources/postgres_crud")

local create = [[mutation { createPostgresBranch(input: {
	postgres: "existing", branch: "restore", sourceBranch: "main", targetTime: "2026-09-30T14:00:00+02:00",
	environmentName: "dev", teamSlug: "pg-crud-team"
}) { postgresBranch { name state } } }]]

Test.gql("Non-members cannot create Postgres branches", function(t)
	t.addHeader("x-user-email", outsider:email())
	t.query(create)
	t.check { errors = { { locations = NotNull(), path = { "createPostgresBranch" }, message = Contains("You are authenticated") } }, data = Null }
end)

Test.gql("Creating a recovery branch does not activate it", function(t)
	t.addHeader("x-user-email", member:email())
	t.query(create)
	t.check { data = { createPostgresBranch = { postgresBranch = { name = "restore", state = "PROGRESSING" } } } }
	t.query [[{ team(slug: "pg-crud-team") { environment(name: "dev") {
		postgres(name: "existing") { desiredActiveBranch activeBranch { name } }
	} } }]]
	t.check { data = { team = { environment = { postgres = { desiredActiveBranch = Null, activeBranch = Null } } } } }
end)

Test.gql("Repeating an identical recovery request returns the same branch", function(t)
	t.addHeader("x-user-email", member:email())
	t.query(create)
	t.check { data = { createPostgresBranch = { postgresBranch = { name = "restore", state = "PROGRESSING" } } } }
end)

Test.gql("Repeating a recovery request with different settings is rejected", function(t)
	t.addHeader("x-user-email", member:email())
	t.query [[mutation { createPostgresBranch(input: {
		postgres: "existing", branch: "restore", sourceBranch: "main", targetTime: "2026-09-29T12:00:00Z",
		environmentName: "dev", teamSlug: "pg-crud-team"
	}) { postgresBranch { name } } }]]
	t.check { errors = { { locations = NotNull(), path = { "createPostgresBranch" }, message = Contains("different recovery settings") } }, data = Null }
end)

Test.gql("Non-members cannot activate Postgres branches", function(t)
	t.addHeader("x-user-email", outsider:email())
	t.query [[mutation { activatePostgresBranch(input: {
		postgres: "existing", branch: "main", environmentName: "dev", teamSlug: "pg-crud-team"
	}) { postgres { desiredActiveBranch } } }]]
	t.check { errors = { { locations = NotNull(), path = { "activatePostgresBranch" }, message = Contains("You are authenticated") } }, data = Null }
end)

Test.gql("Activating a ready branch reports requested versus observed selection", function(t)
	t.addHeader("x-user-email", member:email())
	t.query [[mutation { activatePostgresBranch(input: {
		postgres: "existing", branch: "main", environmentName: "dev", teamSlug: "pg-crud-team"
	}) { postgres { desiredActiveBranch activeBranch { name } } } }]]
	t.check { data = { activatePostgresBranch = { postgres = { desiredActiveBranch = "main", activeBranch = Null } } } }
	t.query [[{ team(slug: "pg-crud-team") { activityLog(first: 10, filter: { activityTypes: [POSTGRES_UPDATED] }) {
		nodes { resourceName ... on PostgresUpdatedActivityLogEntry { data { updatedFields { field oldValue newValue } } } }
	} } }]]
	t.check { data = { team = { activityLog = { nodes = {
		{ resourceName = "existing", data = { updatedFields = { { field = "activeBranch", oldValue = Null, newValue = "main" } } } },
		{ resourceName = "existing", data = { updatedFields = { { field = "branch/restore", oldValue = Null, newValue = "recovered from main at 2026-09-30T12:00:00Z" } } } },
	} } } } }
end)

Test.gql("Requesting a provisioning branch waits for pgrator to activate it", function(t)
	t.addHeader("x-user-email", member:email())
	t.query [[mutation { activatePostgresBranch(input: {
		postgres: "existing", branch: "restore", environmentName: "dev", teamSlug: "pg-crud-team"
	}) { postgres { desiredActiveBranch activeBranch { name } } } }]]
	t.check { data = { activatePostgresBranch = { postgres = { desiredActiveBranch = "restore", activeBranch = Null } } } }
end)
