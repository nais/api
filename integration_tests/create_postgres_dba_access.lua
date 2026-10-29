local user = User.new("dba-user", "dba@usersen.com")
local nonMember = User.new("non-member", "nonmember@usersen.com")
local team = Team.new("someteamname", "Personal DBA access", "#slack_channel")
team:addMember(user)

local serviceAccount = ServiceAccount.new("dba-service-account", team:slug())
serviceAccount:assignRole("Team owner")

Helper.readK8sResources("k8s_resources/create_postgres_access")

local request = [[
	mutation { createPostgresAccess(input: {
		postgres: "foobar", branch: "main", environmentName: "dev", teamSlug: "someteamname",
		accessLevel: DBA, reason: "Investigating database maintenance", ttl: "30m"
	}) { name expiresAt } }
]]

Test.gql("Service accounts cannot request personal DBA access", function(t)
	t.addHeader("authorization", "Bearer " .. serviceAccount:token())
	t.query(request)
	t.check {
		data = Null,
		errors = { {
			locations = NotNull(),
			message = "You are authenticated, but your account is not authorized to perform this action.",
			path = { "createPostgresAccess" },
		} },
	}
end)

Test.gql("Personal DBA access requires team authorization", function(t)
	t.addHeader("x-user-email", nonMember:email())
	t.query(request)
	t.check {
		data = Null,
		errors = { {
			locations = NotNull(),
			message = Contains('you need the "postgres:access:grant" authorization.'),
			path = { "createPostgresAccess" },
		} },
	}
end)

Test.gql("A team member can request personal DBA access", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(request)
	t.check { data = { createPostgresAccess = { name = "postgres-access-7ca5ed09d19eac69", expiresAt = NotNull() } } }
end)

Test.k8s("Personal DBA access uses the authenticated identity and dba access level", function(t)
	t.check("nais.io/v1", "postgresaccesses", "dev", team:slug(), "postgres-access-7ca5ed09d19eac69", {
		apiVersion = "nais.io/v1",
		kind = "PostgresAccess",
		metadata = Ignore(),
		spec = {
			postgresBranch = "foobar-main-a4f04c0c",
			username = user:email(),
			accessLevel = "dba",
			expiresAt = NotNull(),
		},
	})
end)

Test.gql("Personal DBA access is returned through GraphQL and audited", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug: "someteamname") {
		environment(name: "dev") { postgresAccess(name: "postgres-access-7ca5ed09d19eac69") { accessLevel } }
		activityLog(filter: { activityTypes: [POSTGRES_PERSONAL_ACCESS_CREATED] }) { nodes {
			message
			... on PostgresPersonalAccessCreatedActivityLogEntry { data { username accessLevel expiresAt reason } }
		} }
	} }]]
	t.check { data = { team = {
		environment = { postgresAccess = { accessLevel = "DBA" } },
		activityLog = { nodes = { {
			message = Contains("Requested DBA personal Postgres access for dba@usersen.com"),
			data = {
				username = user:email(), accessLevel = "DBA", expiresAt = NotNull(),
				reason = "Investigating database maintenance",
			},
		} } },
	} } }
end)
