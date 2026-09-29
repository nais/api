local user = User.new("user", "user@usersen.com")
local otherMemberUser = User.new("othermember", "othermember@usersen.com")
local nonMemberUser = User.new("nonmember", "other@user.com")

local mainTeam = Team.new("someteamname", "purpose", "#slack_channel")
mainTeam:addMember(user)
mainTeam:addMember(otherMemberUser)

Helper.readK8sResources("k8s_resources/create_postgres_access")

Test.gql("Create personal postgres access without authorization", function(t)
	t.addHeader("x-user-email", nonMemberUser:email())
	t.query [[
		mutation CreatePostgresAccess {
			createPostgresAccess(input: {
				postgresInstance: "foobar"
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
				postgresInstance: "foobar"
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
				postgresInstance: "unknown"
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
				extensions = { field = "postgresInstance" },
				message = Contains("Could not find PostgresInstance"),
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
			postgresInstance: "legacy-only", environmentName: "dev",
			teamSlug: "someteamname", accessLevel: READ,
			reason: "Testing missing physical database instance"
		}) { name } }
	]]
	t.check {
		errors = { { extensions = { field = "postgresInstance" }, message = Contains("Could not find PostgresInstance"), path = { "createPostgresAccess" } } },
		data = Null,
	}
end)

Test.gql("Create personal postgres access rejects an unavailable instance", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation CreatePostgresAccess {
			createPostgresAccess(input: {
				postgresInstance: "progressing"
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
				extensions = { field = "postgresInstance" },
				message = Contains("is not available"),
				path = { "createPostgresAccess" },
			},
		},
		data = Null,
	}
end)

Test.gql("Create personal postgres access", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation CreatePostgresAccess {
			createPostgresAccess(input: {
				postgresInstance: "foobar"
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
	t.addHeader("x-user-email", user:email())
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
							message = Contains("Requested READWRITE personal Postgres access for user@usersen.com"),
							data = {
								username = "user@usersen.com",
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

Test.gql("Personal access targets a physical instance, even when its name differs from logical Postgres", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation { createPostgresAccess(input: {
			postgresInstance: "foobar-recovered", environmentName: "dev",
			teamSlug: "someteamname", accessLevel: READ,
			reason: "Testing access to a recovered instance"
		}) { name } }
	]]
	t.check { data = { createPostgresAccess = { name = NotNull() } } }

	t.query [[
		query { team(slug: "someteamname") { environment(name: "dev") {
			postgresAccess(name: "recovered-access") { postgresInstance { name } }
		} } }
	]]
	t.check { data = { team = { environment = { postgresAccess = { postgresInstance = { name = "foobar-recovered" } } } } } }
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
				serverName = "pg-foobar-rw.someteamname.svc.cluster.local",
				username = "user-foobar-role",
				relayEndpoint = Contains("https://relay.external.dev."),
				relayAccess = "someteamname/ready-access",
				relayToken = "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8",
			},
		} } } },
	}
end)

Test.gql("PostgresAccess connection rejects a different team member", function(t)
	t.addHeader("x-user-email", otherMemberUser:email())
	t.query [[
		query { team(slug: "someteamname") { environment(name: "dev") { postgresAccess(name: "ready-access") { connection { password } } } } }
	]]
	t.check {
		errors = { { locations = NotNull(), path = { "team", "environment", "postgresAccess", "connection" }, message = Contains("not authorized") } },
		data = Null,
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
			errors = { { path = { "viewSecretValues" }, message = Contains("only available through postgresAccessConnection") } },
			data = Null,
		}
	end
end)

Test.gql("PostgresAccess connection rejects expired, unready, and missing-secret access", function(t)
	t.addHeader("x-user-email", user:email())
	for _, test in ipairs({
		{ name = "expired-access",        message = "has expired" },
		{ name = "pending-access",        message = "is not ready" },
		{ name = "failed-access",         message = "is not ready" },
		{ name = "missing-secret-access", message = "secrets for PostgresAccess is not available" },
	}) do
		t.query(string.format(
			[[query { team(slug: "someteamname") { environment(name: "dev") { postgresAccess(name: "%s") { connection { password } } } } }]],
			test.name))
		t.check {
			errors = { { locations = NotNull(), path = { "team", "environment", "postgresAccess", "connection" }, message = Contains(test.message) } },
			data = Null,
		}
	end
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
