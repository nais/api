local member = User.new("pg-branch-user", "pg-branch-user@usersen.com")
local outsider = User.new("pg-branch-outsider", "pg-branch-outsider@usersen.com")
local team = Team.new("pg-crud-team", "Testing Postgres branch operations", "#postgres-crud")
team:addMember(member)
Helper.readK8sResources("k8s_resources/postgres_crud")

Test.gql("Team Postgres list includes databases without branches and paginates", function(t)
	t.addHeader("x-user-email", member:email())
	t.query [[{ team(slug: "pg-crud-team") { postgreses(first: 1) {
		nodes { name majorVersion teamEnvironment { environment { name } } }
		pageInfo { totalCount hasNextPage endCursor }
		facets { labels { key value count } }
	} } }]]
	t.check { data = { team = { postgreses = {
		nodes = { { name = "branchless", majorVersion = "17", teamEnvironment = { environment = { name = "dev" } } } },
		pageInfo = { totalCount = 2, hasNextPage = true, endCursor = Save("postgresNextPage") },
		facets = { labels = { { key = "usecase", value = "reporting", count = 1 } } },
	} } } }
	t.query(string.format([[{ team(slug: "pg-crud-team") { postgreses(first: 1, after: "%s") {
		nodes { name } pageInfo { totalCount hasNextPage }
	} } }]], State.postgresNextPage))
	t.check { data = { team = { postgreses = {
		nodes = { { name = "existing" } }, pageInfo = { totalCount = 2, hasNextPage = false },
	} } } }
end)

Test.gql("Team Postgres list filters instances by label and environment", function(t)
	t.addHeader("x-user-email", member:email())
	t.query [[{ team(slug: "pg-crud-team") { postgreses(filter: {
		environments: ["dev"], labels: [{ key: "usecase", value: "reporting" }]
	}) { nodes { name } pageInfo { totalCount } } } }]]
	t.check { data = { team = { postgreses = { nodes = { { name = "branchless" } }, pageInfo = { totalCount = 1 } } } } }
end)

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

Test.gql("Repeating a recovery request logs only the original creation", function(t)
	t.addHeader("x-user-email", member:email())
	t.query(create)
	t.check { data = { createPostgresBranch = { postgresBranch = { name = "restore", state = "PROGRESSING" } } } }
	t.query [[{ team(slug: "pg-crud-team") { activityLog(first: 10, filter: { activityTypes: [POSTGRES_BRANCH_CREATED] }) {
		nodes { resourceName message ... on PostgresBranchActivityLogEntry { activityType data { branch sourceBranch targetTime } } }
	} } }]]
	t.check { data = { team = { activityLog = { nodes = {
		{ resourceName = "existing", message = "Postgres branch created: restore", activityType = "POSTGRES_BRANCH_CREATED", data = { branch = "restore", sourceBranch = "main", targetTime = "2026-09-30T12:00:00Z" } },
	} } } } }
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
	t.query [[{ team(slug: "pg-crud-team") { activityLog(first: 10, filter: { activityTypes: [POSTGRES_BRANCH_ACTIVATED] }) {
		nodes { resourceName message ... on PostgresBranchActivityLogEntry { activityType data { branch } } }
	} } }]]
	t.check { data = { team = { activityLog = { nodes = {
		{ resourceName = "existing", message = "Postgres branch activated: main", activityType = "POSTGRES_BRANCH_ACTIVATED", data = { branch = "main" } },
	} } } } }
end)

Test.gql("Requesting a provisioning branch waits for pgrator to activate it", function(t)
	t.addHeader("x-user-email", member:email())
	t.query [[mutation { activatePostgresBranch(input: {
		postgres: "existing", branch: "restore", environmentName: "dev", teamSlug: "pg-crud-team"
	}) { postgres { desiredActiveBranch activeBranch { name } } } }]]
	t.check { data = { activatePostgresBranch = { postgres = { desiredActiveBranch = "restore", activeBranch = Null } } } }
end)

Test.gql("Deleting a branch logs its name without deleting the Postgres", function(t)
	t.addHeader("x-user-email", member:email())
	t.query [[mutation { deletePostgresBranch(input: {
		postgres: "existing", branch: "main", environmentName: "dev", teamSlug: "pg-crud-team"
	}) { postgresBranchDeleted } }]]
	t.check { data = { deletePostgresBranch = { postgresBranchDeleted = true } } }
	t.query [[{ team(slug: "pg-crud-team") { activityLog(first: 10, filter: { activityTypes: [POSTGRES_BRANCH_DELETED] }) {
		nodes { resourceName message ... on PostgresBranchActivityLogEntry { activityType data { branch } } }
	} } }]]
	t.check { data = { team = { activityLog = { nodes = {
		{ resourceName = "existing", message = "Postgres branch deleted: main", activityType = "POSTGRES_BRANCH_DELETED", data = { branch = "main" } },
	} } } } }
end)
