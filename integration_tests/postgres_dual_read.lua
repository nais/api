local user = User.new("pg-dual-read-user", "pg-dual-read-user@usersen.com")
local team = Team.new("pg-dual-read-team", "Postgres transition compatibility", "#postgres")
team:addMember(user)
Helper.readK8sResources("k8s_resources/postgres_dual_read")

Test.gql("Direct Postgres fields read both legacy and directly owned databases", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug: "pg-dual-read-team") { postgreses { nodes {
		name state configuration { parameters { name value } } clusterName
	} } } }]]
	t.check { data = { team = { postgreses = { nodes = {
		{ name = "foobar", state = "AVAILABLE", configuration = { parameters = { { name = "work_mem", value = "8MB" } } },  clusterName = "pg-foobar-main-a4f04c0c" },
		{ name = "orders", state = "AVAILABLE", configuration = { parameters = { { name = "work_mem", value = "16MB" } } }, clusterName = "pg-orders-main-65648812" },
	} } } } }
end)

Test.gql("Legacy main queries still work without a Kubernetes branch", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug: "pg-dual-read-team") { environment(name: "dev") { postgres(name: "orders") {
		id createdAt earliestRestoreTime issues { pageInfo { totalCount } } activeBranch { id name state }
		branch(name: "main") { id name state postgres { id name } }
		branches { pageInfo { totalCount } nodes { id name state } }
	} } } }]]
	t.check { data = { team = { environment = { postgres = {
		id = Save("directPostgresID"), createdAt = "2026-01-02T03:04:05Z", earliestRestoreTime = Null, issues = { pageInfo = { totalCount = 0 } },
		activeBranch = { id = Save("directMainID"), name = "main", state = "AVAILABLE" },
		branch = { id = NotNull(), name = "main", state = "AVAILABLE", postgres = { id = NotNull(), name = "orders" } },
		branches = { pageInfo = { totalCount = 1 }, nodes = { { id = NotNull(), name = "main", state = "AVAILABLE" } } },
	} } } } }
end)

Test.gql("The legacy main node keeps its identity and GraphQL type", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[{ node(id: "%s") { __typename ... on PostgresBranch {
		id name postgres { id name } configuration { parameters { name value } }
	} } }]], State.directMainID))
	t.check { data = { node = {
		__typename = "PostgresBranch", id = State.directMainID, name = "main",
		postgres = { id = State.directPostgresID, name = "orders" },
		configuration = { parameters = { { name = "work_mem", value = "16MB" } } },
	} } }
end)

Test.gql("Workload queries resolve a virtual main without omitting the database", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug: "pg-dual-read-team") { environment(name: "dev") { application(name: "consumer") {
		postgreses { nodes { postgres { name state } branch { name state } } }
	} } } }]]
	t.check { data = { team = { environment = { application = { postgreses = { nodes = {
		{ postgres = { name = "orders", state = "AVAILABLE" }, branch = { name = "main", state = "AVAILABLE" } },
	} } } } } } }
end)

Test.gql("Workload nodes reuse the already resolved parent Postgres", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug: "pg-dual-read-team") { environment(name: "dev") { postgres(name: "orders") {
		id workloads { nodes { postgres { id name state } workload { ... on Application { name } } } }
	} } } }]]
	t.check { data = { team = { environment = { postgres = {
		id = State.directPostgresID,
		workloads = { nodes = { { postgres = { id = State.directPostgresID, name = "orders", state = "AVAILABLE" }, workload = { name = "consumer" } } } },
	} } } } }
end)

Test.gql("Personal access can omit branch before and after direct ownership", function(t)
	t.addHeader("x-user-email", user:email())
	for _, postgresName in ipairs({ "foobar", "orders" }) do
		t.query(string.format([[mutation { createPostgresAccess(input: {
			postgres: "%s", teamSlug: "pg-dual-read-team", environmentName: "dev",
			accessLevel: READ, reason: "Testing branchless personal database access", ttl: "30m"
		}) { name expiresAt } }]], postgresName))
		t.check { data = { createPostgresAccess = { name = Save("personalAccessName"), expiresAt = Save("personalAccessExpiry") } } }
		-- Explicit legacy main and omitted branch refer to the same live grant.
		t.query(string.format([[mutation { createPostgresAccess(input: {
			postgres: "%s", branch: "main", teamSlug: "pg-dual-read-team", environmentName: "dev",
			accessLevel: READ, reason: "Testing branchless personal database access"
		}) { name expiresAt } }]], postgresName))
		t.check { data = { createPostgresAccess = { name = State.personalAccessName, expiresAt = State.personalAccessExpiry } } }
		t.query(string.format([[{ team(slug: "pg-dual-read-team") { environment(name: "dev") {
			postgresAccess(name: "%s") { state postgres { name } postgresBranch { name postgres { name } } }
		} } }]], State.personalAccessName))
		t.check { data = { team = { environment = { postgresAccess = {
			state = "PENDING", postgres = { name = postgresName }, postgresBranch = { name = "main", postgres = { name = postgresName } },
		} } } } }
	end
end)
