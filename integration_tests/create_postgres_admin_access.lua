local user = User.new("admin-user", "admin@usersen.com")
local nonMember = User.new("non-member", "nonmember@usersen.com")
local team = Team.new("someteamname", "Personal database administration", "#slack_channel")
team:addMember(user)

local serviceAccount = ServiceAccount.new("admin-service-account", team:slug())
serviceAccount:assignRole("Team owner")

Helper.readK8sResources("k8s_resources/create_postgres_access")

local request = [[
	mutation { createPostgresAccess(input: {
		postgres: "foobar", branch: "main", environmentName: "dev", teamSlug: "someteamname",
		accessLevel: ADMIN, reason: "Running application schema migrations", ttl: "30m"
	}) { name expiresAt } }
]]

Test.gql("Service accounts cannot request personal admin access", function(t)
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

Test.gql("Personal admin access requires team authorization", function(t)
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

Test.gql("A team member can request personal admin access", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(request)
	t.check { data = { createPostgresAccess = { name = "postgres-access-23d4f361acc76446", expiresAt = NotNull() } } }
end)

Test.k8s("Personal admin access preserves the authenticated identity", function(t)
	t.check("nais.io/v1", "postgresaccesses", "dev", team:slug(), "postgres-access-23d4f361acc76446", {
		apiVersion = "nais.io/v1",
		kind = "PostgresAccess",
		metadata = Ignore(),
		spec = {
			postgresBranch = "foobar-main-a4f04c0c",
			username = user:email(),
			accessLevel = "admin",
			expiresAt = NotNull(),
		},
	})
end)

Test.gql("Personal admin access is returned and audited separately from readwritecreate", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug: "someteamname") {
		environment(name: "dev") { postgresAccess(name: "postgres-access-23d4f361acc76446") { accessLevel } }
		activityLog(filter: { activityTypes: [POSTGRES_PERSONAL_ACCESS_CREATED] }) { nodes {
			message
			... on PostgresPersonalAccessCreatedActivityLogEntry { data { username accessLevel expiresAt reason } }
		} }
	} }]]
	t.check { data = { team = {
		environment = { postgresAccess = { accessLevel = "ADMIN" } },
		activityLog = { nodes = { {
			message = Contains("Requested ADMIN personal Postgres access for admin@usersen.com"),
			data = {
				username = user:email(), accessLevel = "ADMIN", expiresAt = NotNull(),
				reason = "Running application schema migrations",
			},
		} } },
	} } }
end)

Test.gql("An active admin request is not reused for readwritecreate", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[mutation { createPostgresAccess(input: {
		postgres: "foobar", branch: "main", environmentName: "dev", teamSlug: "someteamname",
		accessLevel: READWRITECREATE, reason: "Creating personal database objects"
	}) { name } }]]
	t.check {
		data = Null,
		errors = { {
			locations = NotNull(), path = { "createPostgresAccess" },
			message = Contains("already have admin access"),
		} },
	}
end)
