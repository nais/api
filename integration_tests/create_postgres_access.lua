local user = User.new("user", "user@usersen.com")
local otherMemberUser = User.new("othermember", "othermember@usersen.com")
local creatorUser = User.new("creator", "creator@usersen.com")
local staleUser = User.new("stale", "stale@usersen.com")
local nonMemberUser = User.new("nonmember", "other@user.com")

local mainTeam = Team.new("someteamname", "purpose", "#slack_channel")
mainTeam:addMember(user)
mainTeam:addMember(otherMemberUser)
mainTeam:addMember(creatorUser)
mainTeam:addMember(staleUser)

Helper.readK8sResources("k8s_resources/create_postgres_access")

Test.gql("Postgres branches use local names and stay scoped to their Postgres", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug: "someteamname") { environment(name: "dev") {
		postgres(name: "foobar") { activeBranch { name } branches { nodes { name postgres { name } } } branch(name: "recovered") { name postgres { name } } }
	} } }]]
	t.check { data = { team = { environment = { postgres = {
		activeBranch = { name = "main" },
		branches = { nodes = { { name = "main", postgres = { name = "foobar" } }, { name = "recovered", postgres = { name = "foobar" } } } },
		branch = { name = "recovered", postgres = { name = "foobar" } },
	} } } } }
end)

Test.gql("Create personal postgres access without authorization", function(t)
	t.addHeader("x-user-email", nonMemberUser:email())
	t.query [[
		mutation CreatePostgresAccess {
			createPostgresAccess(input: {
				postgres: "foobar", branch: "main"
				environmentName: "dev"
				teamSlug: "someteamname"
				accessLevel: READ
				reason: "Testing personal database access"
			}) {
				name
				expiresAt
			}
		}
	]]

	t.check {
		errors = {
			{
				locations = NotNull(),
				message = Contains('you need the "postgres:access:grant" authorization.'),
				path = { "createPostgresAccess" },
			},
		},
		data = Null,
	}
end)

Test.gql("Create personal postgres access requires an audit reason", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation CreatePostgresAccess {
			createPostgresAccess(input: {
				postgres: "foobar", branch: "main"
				environmentName: "dev"
				teamSlug: "someteamname"
				accessLevel: READ
				reason: "short"
			}) {
				name
			}
		}
	]]

	t.check {
		errors = {
			{
				extensions = { field = "reason" },
				message = Contains("Reason must be at least 10 characters"),
				path = { "createPostgresAccess" },
			},
		},
		data = Null,
	}
end)

Test.gql("Create personal postgres access rejects an unknown instance", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation CreatePostgresAccess {
			createPostgresAccess(input: {
				postgres: "foobar", branch: "unknown"
				environmentName: "dev"
				teamSlug: "someteamname"
				accessLevel: READ
				reason: "Testing personal database access"
			}) {
				name
				expiresAt
			}
		}
	]]

	t.check {
		errors = {
			{
				extensions = { field = "branch" },
				message = Contains("Could not find PostgresBranch"),
				path = { "createPostgresAccess" },
			},
		},
		data = Null,
	}
end)

Test.gql("Create personal postgres access rejects a logical Postgres without a physical instance", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation { createPostgresAccess(input: {
			postgres: "legacy-only", branch: "main", environmentName: "dev",
			teamSlug: "someteamname", accessLevel: READ,
			reason: "Testing missing physical database instance"
		}) { name } }
	]]
	t.check {
		errors = { { extensions = { field = "branch" }, message = Contains("Could not find PostgresBranch"), path = { "createPostgresAccess" } } },
		data = Null,
	}
end)

Test.gql("Create personal postgres access rejects an unavailable instance", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation CreatePostgresAccess {
			createPostgresAccess(input: {
				postgres: "progressing", branch: "main"
				environmentName: "dev"
				teamSlug: "someteamname"
				accessLevel: READ
				reason: "Testing personal database access"
			}) {
				name
				expiresAt
			}
		}
	]]

	t.check {
		errors = {
			{
				extensions = { field = "branch" },
				message = Contains("is not available"),
				path = { "createPostgresAccess" },
			},
		},
		data = Null,
	}
end)

Test.gql("Create personal postgres access", function(t)
	t.addHeader("x-user-email", creatorUser:email())
	t.query [[
		mutation CreatePostgresAccess {
			createPostgresAccess(input: {
				postgres: "foobar", branch: "main"
				environmentName: "dev"
				teamSlug: "someteamname"
				accessLevel: READWRITE
				reason: "Testing personal database access"
				ttl: "30m"
			}) {
				name
				expiresAt
			}
		}
	]]

	t.check {
		data = {
			createPostgresAccess = {
				name = NotNull(),
				expiresAt = NotNull(),
			},
		},
	}
end)

Test.gql("Personal postgres access is audited as a self-grant", function(t)
	t.addHeader("x-user-email", creatorUser:email())
	t.query [[
		{
			team(slug: "someteamname") {
				activityLog {
					nodes {
						message
						... on PostgresPersonalAccessCreatedActivityLogEntry {
							data {
								username
								accessLevel
								expiresAt
								reason
							}
						}
					}
				}
			}
		}
	]]

	t.check {
		data = {
			team = {
				activityLog = {
					nodes = {
						{
							message = Contains("Requested READWRITE personal Postgres access for creator@usersen.com"),
							data = {
								username = "creator@usersen.com",
								accessLevel = "READWRITE",
								expiresAt = NotNull(),
								reason = "Testing personal database access",
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("Requesting access again returns the live access instead of creating another", function(t)
	t.addHeader("x-user-email", creatorUser:email())
	local request = [[
		mutation { createPostgresAccess(input: {
			postgres: "foobar", branch: "main", environmentName: "dev", teamSlug: "someteamname",
			accessLevel: READWRITE, reason: "Testing personal database access"
		}) { name expiresAt } }
	]]
	t.query(request)
	t.check { data = { createPostgresAccess = { name = NotNull(), expiresAt = NotNull() } } }
	t.query(request)
	t.check { data = { createPostgresAccess = { name = NotNull(), expiresAt = NotNull() } } }

	-- A reused access is not a new request, so it must not be audited again.
	t.query [[{ team(slug: "someteamname") { activityLog(filter: { activityTypes: [POSTGRES_PERSONAL_ACCESS_CREATED] }) { nodes { message } } } }]]
	t.check { data = { team = { activityLog = { nodes = { { message = Contains("creator@usersen.com") } } } } } }
end)

Test.gql("Requesting a different access level while one is live is rejected", function(t)
	t.addHeader("x-user-email", creatorUser:email())
	t.query [[
		mutation { createPostgresAccess(input: {
			postgres: "foobar", branch: "main", environmentName: "dev", teamSlug: "someteamname",
			accessLevel: READ, reason: "Testing personal database access"
		}) { name } }
	]]
	t.check {
		errors = { { locations = NotNull(), path = { "createPostgresAccess" }, message = Contains("already have readwrite access") } },
		data = Null,
	}
end)

Test.gql("Personal access targets a physical instance, even when its name differs from logical Postgres", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation { createPostgresAccess(input: {
			postgres: "foobar", branch: "recovered", environmentName: "dev",
			teamSlug: "someteamname", accessLevel: READ,
			reason: "Testing access to a recovered instance"
		}) { name } }
	]]
	t.check { data = { createPostgresAccess = { name = NotNull() } } }

	t.query [[
		query { team(slug: "someteamname") { environment(name: "dev") {
			postgresAccess(name: "recovered-access") { postgresBranch { name } }
		} } }
	]]
	t.check { data = { team = { environment = { postgresAccess = { postgresBranch = { name = "recovered" } } } } } }
end)

Test.gql("PostgresAccess status is visible to authorized team members", function(t)
	t.addHeader("x-user-email", otherMemberUser:email())
	for _, test in ipairs({
		{ name = "ready-access",   state = "READY",   message = "database role and relay mapping are ready" },
		{ name = "pending-access", state = "PENDING", message = Null },
		{ name = "failed-access",  state = "FAILED",  message = Contains("not supported") },
		{ name = "expired-access", state = "EXPIRED", message = "access has expired" },
	}) do
		t.query(string.format(
			[[query { team(slug: "someteamname") { environment(name: "dev") { postgresAccess(name: "%s") { name state message } } } }]],
			test.name))
		t.check {
			data = { team = { environment = {
				postgresAccess = {
					name = test.name,
					state = test.state,
					message = test.message,
				},
			} } },
		}
	end
end)

Test.gql("PostgresAccess status rejects users outside the team", function(t)
	t.addHeader("x-user-email", nonMemberUser:email())
	t.query [[
		query { team(slug: "someteamname") { environment(name: "dev") { postgresAccess(name: "ready-access") { state } } } }
	]]
	t.check {
		errors = {
			{
				locations = NotNull(),
				message = Contains('you need the "postgres:access:grant" authorization.'),
				path = { "team", "environment", "postgresAccess" },
			},
		},
		data = Null,
	}
end)

Test.gql("PostgresAccess connection returns credentials only to its owner", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		query GetPostgresAccessConnection {
			team(slug: "someteamname") { environment(name: "dev") {
				postgresAccess(name: "ready-access") { connection {
					password
					caCertificate
					serverName
					username
					relayEndpoint
					relayAccess
					relayToken
				} }
			} }
		}
	]]

	t.check {
		data = { team = { environment = { postgresAccess = {
			connection = {
				password = "supersecret",
				caCertificate = "test-ca-certificate",
				serverName = "pg-foobar-main-a4f04c0c-rw.someteamname.svc.cluster.local",
				username = "user-foobar-role",
				relayEndpoint = "https://relay.external.dev.nav.cloud.nais.io:8443",
				relayAccess = "someteamname/mapped-access",
				relayToken = "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8",
			},
		} } } },
	}
end)

Test.gql("PostgresAccess connection rejects a different team member", function(t)
	t.addHeader("x-user-email", otherMemberUser:email())
	t.query [[
		query { team(slug: "someteamname") { environment(name: "dev") { postgresAccess(name: "ready-access") { state connection { password } } } } }
	]]
	t.check {
		errors = { { locations = NotNull(), path = { "team", "environment", "postgresAccess", "connection" }, message = Contains("not authorized") } },
		data = { team = { environment = { postgresAccess = { state = "READY", connection = Null } } } },
	}
end)

Test.gql("PostgresAccess credentials cannot be read through generic Secret elevation", function(t)
	t.addHeader("x-user-email", otherMemberUser:email())
	for _, name in ipairs({ "ready-access-relay-token", "ready-access-credentials" }) do
		t.query(string.format([[
			mutation { viewSecretValues(input: {
				name: "%s", team: "someteamname", environment: "dev",
				reason: "Trying to read personal access credentials"
			}) { values { name value } } }
		]], name))
		t.check {
			errors = { { locations = NotNull(), path = { "viewSecretValues" }, message = Contains("only available through postgresAccessConnection") } },
			data = Null,
		}
	end
end)

Test.gql("PostgresAccess connection is null without error until the access is ready", function(t)
	t.addHeader("x-user-email", user:email())
	for _, test in ipairs({
		{ name = "expired-access", state = "EXPIRED" },
		{ name = "pending-access", state = "PENDING" },
		{ name = "failed-access",  state = "FAILED" },
	}) do
		t.query(string.format(
			[[query { team(slug: "someteamname") { environment(name: "dev") { postgresAccess(name: "%s") { state connection { password } } } } }]],
			test.name))
		t.check {
			data = { team = { environment = { postgresAccess = { state = test.state, connection = Null } } } },
		}
	end
end)

Test.gql("PostgresAccess connection is null before relay endpoint is published", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[query { team(slug: "someteamname") { environment(name: "dev") { postgresAccess(name: "ready-no-endpoint-access") { connection { relayEndpoint } } } } }]]
	t.check { data = { team = { environment = { postgresAccess = { connection = Null } } } } }
end)

Test.gql("PostgresAccess connection fails when a ready access has no secrets", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[query { team(slug: "someteamname") { environment(name: "dev") { postgresAccess(name: "missing-secret-access") { state connection { password } } } } }]]
	t.check {
		errors = { { locations = NotNull(), path = { "team", "environment", "postgresAccess", "connection" }, message = Contains("secrets for PostgresAccess is not available") } },
		data = { team = { environment = { postgresAccess = { state = "READY", connection = Null } } } },
	}
end)

Test.gql("Personal postgres connection retrieval is audited", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		query { team(slug: "someteamname") { environment(name: "dev") { postgresAccess(name: "ready-access") { connection { password } } } } }
	]]
	t.check { data = { team = { environment = { postgresAccess = { connection = { password = "supersecret" } } } } } }

	t.query [[
		{ team(slug: "someteamname") { activityLog(first: 1) { nodes { message ... on PostgresPersonalAccessConnectionActivityLogEntry { resourceName } } } } }
	]]
	t.check {
		data = { team = { activityLog = { nodes = {
			{ message = Contains("Retrieved personal Postgres connection materials"), resourceName = "ready-access" },
		} } } },
	}
end)

Test.gql("An expired access is cleaned up, after which a new one can be requested", function(t)
	t.addHeader("x-user-email", staleUser:email())
	local request = [[
		mutation { createPostgresAccess(input: {
			postgres: "foobar", branch: "main", environmentName: "dev", teamSlug: "someteamname",
			accessLevel: READ, reason: "Testing personal database access"
		}) { name } }
	]]
	t.query(request)
	t.check {
		errors = { { locations = NotNull(), path = { "createPostgresAccess" }, message = Contains("being cleaned up") } },
		data = Null,
	}

	t.query(request)
	t.check { data = { createPostgresAccess = { name = "postgres-access-9269571e872b7c9d" } } }
end)
