local user = User.new("user", "user@usersen.com")
local nonMember = User.new("nonmember", "not@usersen.com")
local team = Team.new("someteamname", "purpose", "#slack_channel")
team:addMember(user)
Helper.readK8sResources("k8s_resources/postgres_branches")

Test.gql("List concrete PostgresBranches with their logical owners", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug: "someteamname") { postgresBranches(orderBy: {field: NAME, direction: ASC}) {
        nodes { name state postgres { name majorVersion activeBranch { name } } }
    } } }]]
	t.check { data = { team = { postgresBranches = { nodes = {
		{ name = "main", state = "AVAILABLE", postgres = { name = "another-db", majorVersion = "16", activeBranch = { name = "main" } } },
		{ name = "main", state = "AVAILABLE", postgres = { name = "foobar", majorVersion = "17", activeBranch = { name = "main" } } },
		{ name = "main", state = "AVAILABLE", postgres = { name = "with-audit", majorVersion = "16", activeBranch = { name = "main" } } },
		{ name = "main", state = "AVAILABLE", postgres = { name = "without-audit", majorVersion = "15", activeBranch = { name = "main" } } },
	} } } } }
end)

Test.gql("Retrieve logical Postgres and selected physical instance separately", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug:"someteamname") { environment(name:"dev") {
        postgres(name:"foobar") { name activeBranch { name } majorVersion highAvailability resources { cpu memory diskSize } branch(name:"main") { name postgres { name } teamEnvironment { name } } branches { nodes { name } } }
    } } }]]
	t.check { data = { team = { environment = {
		postgres = { name = "foobar", activeBranch = { name = "main" }, majorVersion = "17", highAvailability = false, resources = { cpu = "100m", memory = "2Gi", diskSize = "2Gi" }, branch = { name = "main", postgres = { name = "foobar" }, teamEnvironment = { name = "dev" } }, branches = { nodes = { { name = "main" } } } },
	} } } }
end)

Test.gql("Physical instances may be filtered by observed state", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug:"someteamname") { postgresBranches(filter: {states:[AVAILABLE]}) {
        nodes { name } facets { states { state count } }
    } } }]]
	t.check { data = { team = { postgresBranches = {
		nodes = { { name = "main" }, { name = "main" }, { name = "main" }, { name = "main" } },
		facets = { states = { { state = "AVAILABLE", count = 4 } } },
	} } } }
end)

Test.gql("A workload follows the active physical instance", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug:"someteamname") { environment(name:"dev") {
        application(name:"app-with-postgres") { postgresBranches { nodes { name postgres { name } } } }
        postgres(name:"foobar") { branch(name:"main") { workloads { nodes { __typename name } } } }
    } } }]]
	t.check { data = { team = { environment = {
		application = { postgresBranches = { nodes = { { name = "main", postgres = { name = "foobar" } } } } },
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
