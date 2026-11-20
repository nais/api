local user = User.new("pg-dual-read-user", "pg-dual-read-user@usersen.com")
local team = Team.new("pg-dual-read-team", "Postgres transition compatibility", "#postgres")
team:addMember(user)
Helper.readK8sResources("k8s_resources/postgres_dual_read")

Test.gql("Postgres fields read database-owned status", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug: "pg-dual-read-team") { postgreses { nodes {
		name state configuration { parameters { name value } }
	} } } }]]
	t.check { data = { team = { postgreses = { nodes = {
		{ name = "foobar", state = "AVAILABLE", configuration = { parameters = { { name = "work_mem", value = "8MB" } } } },
		{ name = "orders", state = "AVAILABLE", configuration = { parameters = { { name = "work_mem", value = "16MB" } } } },
	} } } } }
end)

Test.gql("Postgres metadata is read from the database", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug: "pg-dual-read-team") { environment(name: "dev") { postgres(name: "orders") {
		id name state createdAt earliestRestoreTime issues { pageInfo { totalCount } }
	} } } }]]
	t.check { data = { team = { environment = { postgres = {
		id = Save("directPostgresID"), createdAt = "2026-01-02T03:04:05Z", earliestRestoreTime = Null, issues = { pageInfo = { totalCount = 0 } },
		name = "orders", state = "AVAILABLE",
	} } } } }
end)

Test.gql("The Postgres node keeps its identity and GraphQL type", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[{ node(id: "%s") { __typename ... on Postgres {
		id name configuration { parameters { name value } }
	} } }]], State.directPostgresID))
	t.check { data = { node = {
		__typename = "Postgres", id = State.directPostgresID, name = "orders",
		configuration = { parameters = { { name = "work_mem", value = "16MB" } } },
	} } }
end)

Test.gql("Workload queries resolve their database", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug: "pg-dual-read-team") { environment(name: "dev") { application(name: "consumer") {
		postgreses { nodes { postgres { name state } } }
	} } } }]]
	t.check { data = { team = { environment = { application = { postgreses = { nodes = {
		{ postgres = { name = "orders", state = "AVAILABLE" } },
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

Test.gql("Personal access is scoped to its database and reused", function(t)
	t.addHeader("x-user-email", user:email())
	for _, postgresName in ipairs({ "foobar", "orders" }) do
		t.query(string.format([[mutation { createPostgresAccess(input: {
			postgres: "%s", teamSlug: "pg-dual-read-team", environmentName: "dev",
			accessLevel: READ, reason: "Testing personal database access", ttl: "30m"
		}) { name expiresAt } }]], postgresName))
		t.check { data = { createPostgresAccess = { name = Save("personalAccessName"), expiresAt = Save("personalAccessExpiry") } } }
		-- Repeating the request returns the same live grant.
		t.query(string.format([[mutation { createPostgresAccess(input: {
			postgres: "%s", teamSlug: "pg-dual-read-team", environmentName: "dev",
			accessLevel: READ, reason: "Testing personal database access"
		}) { name expiresAt } }]], postgresName))
		t.check { data = { createPostgresAccess = { name = State.personalAccessName, expiresAt = State.personalAccessExpiry } } }
		t.query(string.format([[{ team(slug: "pg-dual-read-team") { environment(name: "dev") {
			postgresAccess(name: "%s") { state postgres { name } }
		} } }]], State.personalAccessName))
		t.check { data = { team = { environment = { postgresAccess = {
			state = "PENDING", postgres = { name = postgresName },
		} } } } }
	end
end)
