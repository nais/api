local user = User.new("user", "user@usersen.com")
local nonMember = User.new("nonmember", "not@usersen.com")
local team = Team.new("someteamname", "purpose", "#slack_channel")
team:addMember(user)
Helper.readK8sResources("k8s_resources/postgres_instances")

Test.gql("List concrete PostgresInstances with their logical owners", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug: "someteamname") { postgresInstances(orderBy: {field: NAME, direction: ASC}) {
        nodes { name state postgres { name majorVersion activeInstance } }
    } } }]]
	t.check { data = { team = { postgresInstances = { nodes = {
		{ name = "another-db",    state = "AVAILABLE", postgres = { name = "another-db", majorVersion = "16", activeInstance = "another-db" } },
		{ name = "foobar",        state = "AVAILABLE", postgres = { name = "foobar", majorVersion = "17", activeInstance = "foobar" } },
		{ name = "with-audit",    state = "AVAILABLE", postgres = { name = "with-audit", majorVersion = "16", activeInstance = "with-audit" } },
		{ name = "without-audit", state = "AVAILABLE", postgres = { name = "without-audit", majorVersion = "15", activeInstance = "without-audit" } },
	} } } } }
end)

Test.gql("Retrieve logical Postgres and selected physical instance separately", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug:"someteamname") { environment(name:"dev") {
        postgres(name:"foobar") { name activeInstance majorVersion highAvailability resources { cpu memory diskSize } }
        postgresInstance(name:"foobar") { name postgres { name } teamEnvironment { name } }
    } } }]]
	t.check { data = { team = { environment = {
		postgres = { name = "foobar", activeInstance = "foobar", majorVersion = "17", highAvailability = false, resources = { cpu = "100m", memory = "2Gi", diskSize = "2Gi" } },
		postgresInstance = { name = "foobar", postgres = { name = "foobar" }, teamEnvironment = { name = "dev" } },
	} } } }
end)

Test.gql("Physical instances may be filtered by observed state", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug:"someteamname") { postgresInstances(filter: {states:[AVAILABLE]}) {
        nodes { name } facets { states { state count } }
    } } }]]
	t.check { data = { team = { postgresInstances = {
		nodes = { { name = "another-db" }, { name = "foobar" }, { name = "with-audit" }, { name = "without-audit" } },
		facets = { states = { { state = "AVAILABLE", count = 4 } } },
	} } } }
end)

Test.gql("A workload follows the active physical instance", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug:"someteamname") { environment(name:"dev") {
        application(name:"app-with-postgres") { postgresInstances { nodes { name postgres { name } } } }
        postgresInstance(name:"foobar") { workloads { nodes { __typename name } } }
    } } }]]
	t.check { data = { team = { environment = {
		application = { postgresInstances = { nodes = { { name = "foobar", postgres = { name = "foobar" } } } } },
		postgresInstance = { workloads = { nodes = {
			{ __typename = "Application", name = "app-with-postgres" },
			{ __typename = "Application", name = "app-with-postgres-2" },
			{ __typename = "Job",         name = "job-with-postgres" },
		} } },
	} } } }
end)

Test.gql("Delete a concrete PostgresInstance requires authorization", function(t)
	t.addHeader("x-user-email", nonMember:email())
	t.query [[mutation { deletePostgres(input:{name:"foobar",environmentName:"dev",teamSlug:"someteamname"}) { postgresDeleted } }]]
	t.check { errors = { { locations = NotNull(), message = Contains('postgres:delete'), path = { "deletePostgres" } } }, data = Null }
end)
