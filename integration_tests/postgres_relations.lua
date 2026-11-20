local user = User.new("pg-selection-user", "pg-selection-user@usersen.com")
local team = Team.new("pg-selection-team", "Testing Postgres workload relations", "#postgres")
team:addMember(user)
Helper.readK8sResources("k8s_resources/postgres_relations")

Test.gql("Postgres includes all application and job consumers", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug: "pg-selection-team") { environment(name: "dev") {
		postgres(name: "orders") { workloads { nodes { workload { __typename ... on Application { name } ... on Job { name } } } } }
	} } }]]
	t.check { data = { team = { environment = { postgres = { workloads = { nodes = {
		{ workload = { __typename = "Application", name = "first-app" } },
		{ workload = { __typename = "Job", name = "scheduled-reader" } },
		{ workload = { __typename = "Application", name = "second-app" } },
	} } } } } } }
end)

Test.gql("An incomplete physical status remains provisioning", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug: "pg-selection-team") { environment(name: "dev") {
		postgres(name: "orders") { name state }
		application(name: "first-app") { postgreses { nodes { postgres { name } } } }
	} } }]]
	t.check { data = { team = { environment = {
		postgres = { name = "orders", state = "PROGRESSING" },
		application = { postgreses = { nodes = { { postgres = { name = "orders" } } } } },
	} } } }
end)

Test.gql("Postgres pagination is validated when a workload has no uses", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug: "pg-selection-team") { environment(name: "dev") {
		job(name: "no-postgres-uses") { postgreses(first: 0) { pageInfo { totalCount } } }
	} } }]]
	t.check { data = Null, errors = { { locations = NotNull(), message = Contains("first must be greater than or equal to 1"), path = { "team", "environment", "job", "postgreses" } } } }
end)

Test.gql("A provisioning database retains workload relations", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug: "pg-selection-team") { environment(name: "dev") {
		postgres(name: "provisioning") {
			state
			workloads { pageInfo { totalCount } nodes { workload { ... on Application { name } } } }
		}
	} } }]]
	t.check { data = { team = { environment = { postgres = {
		state = "PROGRESSING",
		workloads = { pageInfo = { totalCount = 1 }, nodes = { { workload = { name = "provisioning-app" } } } },
	} } } } }
end)
