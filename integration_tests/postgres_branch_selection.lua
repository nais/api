local user = User.new("pg-selection-user", "pg-selection-user@usersen.com")
local team = Team.new("pg-selection-team", "Testing the retired branch facade", "#postgres")
team:addMember(user)
Helper.readK8sResources("k8s_resources/postgres_branch_selection")

Test.gql("The main facade includes omitted and explicit main consumers", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug: "pg-selection-team") { environment(name: "dev") {
		postgres(name: "orders") { branch(name: "main") { workloads { nodes { __typename name } } } }
	} } }]]
	t.check { data = { team = { environment = { postgres = { branch = { workloads = { nodes = {
		{ __typename = "Application", name = "following-app" },
		{ __typename = "Application", name = "pinned-app" },
		{ __typename = "Job",         name = "pinned-job" },
	} } } } } } } }
end)

Test.gql("Stale active branch metadata never selects a physical target", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug: "pg-selection-team") { environment(name: "dev") {
		postgres(name: "orders") { activeBranch { name } branches { nodes { name } } }
		application(name: "following-app") { postgreses { nodes { branch { name } postgres { name } } } }
	} } }]]
	t.check { data = { team = { environment = {
		postgres = { activeBranch = { name = "main" }, branches = { nodes = { { name = "main" } } } },
		application = { postgreses = { nodes = { { branch = { name = "main" }, postgres = { name = "orders" } } } } },
	} } } }
end)

Test.gql("Retired non-main lookups do not fall back to main", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug: "pg-selection-team") { environment(name: "dev") {
		postgres(name: "orders") { branch(name: "green") { name } }
	} } }]]
	t.check { data = Null, errors = { { locations = NotNull(), message = Contains("orders/green"), path = { "team", "environment", "postgres", "branch" } } } }
end)

Test.gql("Postgres pagination is validated when a workload has no uses", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug: "pg-selection-team") { environment(name: "dev") {
		job(name: "no-postgres-uses") { postgreses(first: 0) { pageInfo { totalCount } } }
	} } }]]
	t.check { data = Null, errors = { { locations = NotNull(), message = Contains("first must be greater than or equal to 1"), path = { "team", "environment", "job", "postgreses" } } } }
end)

Test.gql("A provisioning database retains workload relations without branch objects", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug: "pg-selection-team") { environment(name: "dev") {
		postgres(name: "missing-branch") {
			state branch(name: "main") { name state }
			workloads { pageInfo { totalCount } nodes { branch { name } workload { ... on Application { name } } } }
		}
	} } }]]
	t.check { data = { team = { environment = { postgres = {
		state = "PROGRESSING", branch = { name = "main", state = "PROGRESSING" },
		workloads = { pageInfo = { totalCount = 1 }, nodes = { { branch = { name = "main" }, workload = { name = "missing-branch-app" } } } },
	} } } } }
end)
