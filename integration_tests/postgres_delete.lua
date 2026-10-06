local member = User.new("postgres-delete-whole-member", "postgres-delete-whole-member@usersen.com")
local outsider = User.new("postgres-delete-whole-outsider", "postgres-delete-whole-outsider@usersen.com")
local team = Team.new("pg-delete-whole", "Whole Postgres deletion", "#postgres-delete-whole")
team:addMember(member)
Helper.readK8sResources("k8s_resources/postgres_delete")

local function request(name)
	return string.format(
		[[mutation { deletePostgres(input: { name: "%s", teamSlug: "pg-delete-whole", environmentName: "dev" }) { deletionRequested } }]],
		name)
end

Test.gql("Non-members cannot delete Postgres", function(t)
	t.addHeader("x-user-email", outsider:email())
	t.query(request("removable"))
	t.check { errors = { { locations = NotNull(), path = { "deletePostgres" }, message = Contains("You are authenticated") } }, data = Null }
end)

for _, case in ipairs({
	{ name = "by-app",     reason = 'Application "consumer-app"' },
	{ name = "by-job",     reason = 'Naisjob "consumer-job"' },
	{ name = "by-binding", reason = 'PostgresBinding "consumer-binding"' },
}) do
	Test.gql("Referencing " .. case.reason .. " blocks whole Postgres deletion", function(t)
		t.addHeader("x-user-email", member:email())
		t.query(request(case.name))
		t.check { errors = { { locations = NotNull(), path = { "deletePostgres" }, message = Contains(case.reason) } }, data = Null }
	end)
end

Test.gql("Invalid Postgres names cannot be deleted", function(t)
	t.addHeader("x-user-email", member:email())
	t.query(request("Invalid/Name"))
	t.check { errors = { { path = { "deletePostgres" }, extensions = { field = "name" }, message = Contains("lowercase") } }, data = Null }
end)

Test.gql("Deletion waits for pgrator to install its cleanup finalizer", function(t)
	t.addHeader("x-user-email", member:email())
	t.query(request("missing-finalizer"))
	t.check { errors = { { locations = NotNull(), path = { "deletePostgres" }, message = Contains("deletion finalizer") } }, data = Null }
end)

Test.gql("Deleting a whole Postgres only acknowledges a request", function(t)
	t.addHeader("x-user-email", member:email())
	t.query(request("removable"))
	t.check { data = { deletePostgres = { deletionRequested = true } } }
end)

Test.gql("Postgres deletion request has a distinct audit filter and concrete type", function(t)
	t.addHeader("x-user-email", member:email())
	t.query [[{ team(slug: "pg-delete-whole") { activityLog(first: 20, filter: { activityTypes: [POSTGRES_DELETION_REQUESTED] }) {
		nodes { __typename message resourceName actor }
	} } }]]
	t.check { data = { team = { activityLog = { nodes = {
		{ __typename = "PostgresDeletedActivityLogEntry", message = "Requested deletion of Postgres; cleanup is pending", resourceName = "removable", actor = member:email() },
	} } } } }
end)
