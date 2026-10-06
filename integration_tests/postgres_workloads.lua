local user = User.new("pg-workload-user", "pg-workload-user@usersen.com")
local team = Team.new("postgres-workload-team", "Testing workload Postgres uses", "#postgres-workloads")
team:addMember(user)
Helper.readK8sResources("k8s_resources/postgres_workloads")

Test.gql("Application and job list every referenced Postgres, including one without an active branch", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug: "postgres-workload-team") { environment(name: "dev") {
		application(name: "consumer") { postgreses { nodes { name activeBranch { name } } } }
		job(name: "scheduled-reader") { postgreses { nodes { name activeBranch { name } } } }
	} } }]]
	local databases = {
		{ name = "archive", activeBranch = Null },
		{ name = "orders",  activeBranch = { name = "green" } },
		{ name = "reports", activeBranch = { name = "recovered" } },
	}
	t.check { data = { team = { environment = {
		application = { postgreses = { nodes = databases } },
		job = { postgreses = { nodes = databases } },
	} } } }
end)

Test.gql("PostgresBranch workloads reference its Postgres through uses.postgres", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug: "postgres-workload-team") { environment(name: "dev") {
		postgres(name: "orders") { branch(name: "green") { workloads { nodes { __typename name } } } }
		other: postgres(name: "reports") { branch(name: "recovered") { workloads { nodes { __typename name } } } }
	} } }]]
	local workloads = {
		{ __typename = "Application", name = "consumer" },
		{ __typename = "Job",         name = "scheduled-reader" },
	}
	t.check { data = { team = { environment = {
		postgres = { branch = { workloads = { nodes = workloads } } },
		other = { branch = { workloads = { nodes = workloads } } },
	} } } }
end)
