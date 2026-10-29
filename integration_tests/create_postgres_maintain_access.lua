local user = User.new("maintain-user", "maintain@usersen.com")
local nonMember = User.new("non-member", "nonmember@usersen.com")
local team = Team.new("someteamname", "Personal instance maintenance access", "#slack_channel")
team:addMember(user)

local serviceAccount = ServiceAccount.new("maintain-service-account", team:slug())
serviceAccount:assignRole("Team owner")

Helper.readK8sResources("k8s_resources/create_postgres_access")

local request = [[
	mutation { createPostgresAccess(input: {
		postgres: "foobar", branch: "main", environmentName: "dev", teamSlug: "someteamname",
		accessLevel: MAINTAIN, reason: "Investigating database maintenance", ttl: "30m"
	}) { name expiresAt } }
]]

Test.gql("Service accounts cannot request personal maintain access", function(t)
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

Test.gql("Personal maintain access requires team authorization", function(t)
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

Test.gql("A team member can request personal maintain access", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(request)
	t.check { data = { createPostgresAccess = { name = "postgres-access-420767dcef636fe6", expiresAt = NotNull() } } }
end)

Test.k8s("Personal maintain access uses the authenticated identity and maintain access level", function(t)
	t.check("nais.io/v1", "postgresaccesses", "dev", team:slug(), "postgres-access-420767dcef636fe6", {
		apiVersion = "nais.io/v1",
		kind = "PostgresAccess",
		metadata = Ignore(),
		spec = {
			postgresBranch = "foobar-main-a4f04c0c",
			username = user:email(),
			accessLevel = "maintain",
			expiresAt = NotNull(),
		},
	})
end)

Test.gql("Personal maintain access is returned through GraphQL and audited", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug: "someteamname") {
		environment(name: "dev") { postgresAccess(name: "postgres-access-420767dcef636fe6") { accessLevel } }
		activityLog(filter: { activityTypes: [POSTGRES_PERSONAL_ACCESS_CREATED] }) { nodes {
			message
			... on PostgresPersonalAccessCreatedActivityLogEntry { data { username accessLevel expiresAt reason } }
		} }
	} }]]
	t.check { data = { team = {
		environment = { postgresAccess = { accessLevel = "MAINTAIN" } },
		activityLog = { nodes = { {
			message = Contains("Requested MAINTAIN personal Postgres access for maintain@usersen.com"),
			data = {
				username = user:email(), accessLevel = "MAINTAIN", expiresAt = NotNull(),
				reason = "Investigating database maintenance",
			},
		} } },
	} } }
end)
