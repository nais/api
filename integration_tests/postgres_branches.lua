local user = User.new("user", "user@usersen.com")
local nonMember = User.new("nonmember", "not@usersen.com")
local team = Team.new("someteamname", "purpose", "#slack_channel")
team:addMember(user)
Helper.readK8sResources("k8s_resources/postgres_branches")

Test.gql("List concrete PostgresBranches with their logical owners", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug: "someteamname") { postgresBranches(orderBy: {field: NAME, direction: ASC}) {
        nodes { name state postgres { name majorVersion activeBranch } }
    } } }]]
	t.check { data = { team = { postgresBranches = { nodes = {
		{ name = "another-db",    state = "AVAILABLE", postgres = { name = "another-db", majorVersion = "16", activeBranch = "another-db" } },
		{ name = "foobar",        state = "AVAILABLE", postgres = { name = "foobar", majorVersion = "17", activeBranch = "foobar" } },
		{ name = "with-audit",    state = "AVAILABLE", postgres = { name = "with-audit", majorVersion = "16", activeBranch = "with-audit" } },
		{ name = "without-audit", state = "AVAILABLE", postgres = { name = "without-audit", majorVersion = "15", activeBranch = "without-audit" } },
	} } } } }
end)

Test.gql("Retrieve logical Postgres and selected physical instance separately", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug:"someteamname") { environment(name:"dev") {
        postgres(name:"foobar") { name activeBranch majorVersion highAvailability resources { cpu memory diskSize } }
        postgresBranch(name:"foobar") { name postgres { name } teamEnvironment { name } }
    } } }]]
	t.check { data = { team = { environment = {
		postgres = { name = "foobar", activeBranch = "foobar", majorVersion = "17", highAvailability = false, resources = { cpu = "100m", memory = "2Gi", diskSize = "2Gi" } },
		postgresBranch = { name = "foobar", postgres = { name = "foobar" }, teamEnvironment = { name = "dev" } },
	} } } }
end)

Test.gql("Physical instances may be filtered by observed state", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug:"someteamname") { postgresBranches(filter: {states:[AVAILABLE]}) {
        nodes { name } facets { states { state count } }
    } } }]]
	t.check { data = { team = { postgresBranches = {
		nodes = { { name = "another-db" }, { name = "foobar" }, { name = "with-audit" }, { name = "without-audit" } },
		facets = { states = { { state = "AVAILABLE", count = 4 } } },
	} } } }
end)

Test.gql("A workload follows the active physical instance", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug:"someteamname") { environment(name:"dev") {
        application(name:"app-with-postgres") { postgresBranches { nodes { name postgres { name } } } }
        postgresBranch(name:"foobar") { workloads { nodes { __typename name } } }
    } } }]]
	t.check { data = { team = { environment = {
		application = { postgresBranches = { nodes = { { name = "foobar", postgres = { name = "foobar" } } } } },
		postgresBranch = { workloads = { nodes = {
			{ __typename = "Application", name = "app-with-postgres" },
			{ __typename = "Application", name = "app-with-postgres-2" },
			{ __typename = "Job",         name = "job-with-postgres" },
		} } },
	} } } }
end)

Test.gql("Delete a concrete PostgresBranch requires authorization", function(t)
	t.addHeader("x-user-email", nonMember:email())
	t.query [[mutation { deletePostgresBranch(input:{name:"foobar",environmentName:"dev",teamSlug:"someteamname"}) { postgresBranchDeleted } }]]
	t.check { errors = { { locations = NotNull(), message = Contains('postgres:delete'), path = { "deletePostgresBranch" } } }, data = Null }
end)
