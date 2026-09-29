local user = User.new("pg-workload-user", "pg-workload-user@usersen.com")
local team = Team.new("postgres-workload-team", "Testing workload Postgres uses", "#postgres-workloads")
team:addMember(user)
Helper.readK8sResources("k8s_resources/postgres_workloads")

Test.gql("Application and job resolve every uses.postgres entry to its selected instance", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug: "postgres-workload-team") { environment(name: "dev") {
		application(name: "consumer") { postgresInstances { nodes { name postgres { name } } } }
		job(name: "scheduled-reader") { postgresInstances { nodes { name postgres { name } } } }
	} } }]]
	local instances = {
		{ name = "orders-green",      postgres = { name = "orders" } },
		{ name = "reports-recovered", postgres = { name = "reports" } },
	}
	t.check { data = { team = { environment = {
		application = { postgresInstances = { nodes = instances } },
		job = { postgresInstances = { nodes = instances } },
	} } } }
end)

Test.gql("PostgresInstance workloads reference its Postgres through uses.postgres", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug: "postgres-workload-team") { environment(name: "dev") {
		postgresInstance(name: "orders-green") { workloads { nodes { __typename name } } }
		other: postgresInstance(name: "reports-recovered") { workloads { nodes { __typename name } } }
	} } }]]
	local workloads = {
		{ __typename = "Application", name = "consumer" },
		{ __typename = "Job",         name = "scheduled-reader" },
	}
	t.check { data = { team = { environment = {
		postgresInstance = { workloads = { nodes = workloads } },
		other = { workloads = { nodes = workloads } },
	} } } }
end)
