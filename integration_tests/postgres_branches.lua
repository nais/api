local user = User.new("user", "user@usersen.com")
local nonMember = User.new("nonmember", "not@usersen.com")
local team = Team.new("someteamname", "purpose", "#slack_channel")
team:addMember(user)
Helper.readK8sResources("k8s_resources/postgres_branches")

Test.gql("List Postgres and their branches together", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug: "someteamname") { postgreses { nodes {
        name majorVersion activeBranch { name } branches { nodes { name state postgres { name } } }
    } } } }]]
	t.check { data = { team = { postgreses = { nodes = {
		{ name = "another-db",    majorVersion = "16", activeBranch = { name = "main" }, branches = { nodes = { { name = "main", state = "AVAILABLE", postgres = { name = "another-db" } } } } },
		{ name = "foobar",        majorVersion = "17", activeBranch = { name = "main" }, branches = { nodes = { { name = "main", state = "AVAILABLE", postgres = { name = "foobar" } } } } },
		{ name = "with-audit",    majorVersion = "16", activeBranch = { name = "main" }, branches = { nodes = { { name = "main", state = "AVAILABLE", postgres = { name = "with-audit" } } } } },
		{ name = "without-audit", majorVersion = "15", activeBranch = { name = "main" }, branches = { nodes = { { name = "main", state = "AVAILABLE", postgres = { name = "without-audit" } } } } },
	} } } } }
end)

Test.gql("Retrieve logical Postgres and selected physical instance separately", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug:"someteamname") { environment(name:"dev") {
        postgres(name:"foobar") { name activeBranch { name clusterName } majorVersion highAvailability resources { cpu memory diskSize } branch(name:"main") { name postgres { name } teamEnvironment { name } } branches { nodes { name } } }
    } } }]]
	t.check { data = { team = { environment = {
		postgres = { name = "foobar", activeBranch = { name = "main", clusterName = "pg-foobar-main-a4f04c0c" }, majorVersion = "17", highAvailability = false, resources = { cpu = "100m", memory = "2Gi", diskSize = "2Gi" }, branch = { name = "main", postgres = { name = "foobar" }, teamEnvironment = { name = "dev" } }, branches = { nodes = { { name = "main" } } } },
	} } } }
end)

Test.gql("Branch state facets are scoped to their Postgres", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug:"someteamname") { environment(name:"dev") { postgres(name:"foobar") {
        branches { nodes { name state } facets { states { state count } } }
    } } } }]]
	t.check { data = { team = { environment = { postgres = { branches = {
		nodes = { { name = "main", state = "AVAILABLE" } },
		facets = { states = { { state = "AVAILABLE", count = 1 } } },
	} } } } } }
end)

Test.gql("A workload follows the active physical instance", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug:"someteamname") { environment(name:"dev") {
        application(name:"app-with-postgres") { postgreses { nodes { name activeBranch { name } } } }
        postgres(name:"foobar") { branch(name:"main") { workloads { nodes { __typename name } } } }
    } } }]]
	t.check { data = { team = { environment = {
		application = { postgreses = { nodes = { { name = "foobar", activeBranch = { name = "main" } } } } },
		postgres = { branch = { workloads = { nodes = {
			{ __typename = "Application", name = "app-with-postgres" },
			{ __typename = "Application", name = "app-with-postgres-2" },
			{ __typename = "Job",         name = "job-with-postgres" },
		} } } },
	} } } }
end)

Test.gql("Delete a concrete PostgresBranch requires authorization", function(t)
	t.addHeader("x-user-email", nonMember:email())
	t.query [[mutation { deletePostgresBranch(input:{postgres:"foobar",branch:"main",environmentName:"dev",teamSlug:"someteamname"}) { postgresBranchDeleted } }]]
	t.check { errors = { { locations = NotNull(), message = Contains('postgres:delete'), path = { "deletePostgresBranch" } } }, data = Null }
end)
