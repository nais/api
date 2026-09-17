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
				clientWireGuardPublicKey: "client-public-key"
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
				clientWireGuardPublicKey: "client-public-key"
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
				clientWireGuardPublicKey: "client-public-key"
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
				message = Contains("Could not find postgres cluster"),
				path = { "createPostgresAccess" },
			},
		},
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
				clientWireGuardPublicKey: "client-public-key"
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
				clientWireGuardPublicKey: "client-public-key"
				reason: "Testing personal database access"
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
							message = Contains("Created personal Postgres access for user@usersen.com"),
							data = {
								username = "user@usersen.com",
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

Test.gql("PostgresAccess connection returns credentials only to its owner", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		query GetPostgresAccessConnection {
			postgresAccessConnection(input: {name: "ready-access", teamSlug: "someteamname", environmentName: "dev"}) {
				password
				caCertificate
				serverName
				tunnel {
					endpoint
					gatewayPublicKey
				}
			}
		}
	]]

	t.check {
		data = {
			postgresAccessConnection = {
				password = "supersecret",
				caCertificate = "test-ca-certificate",
				serverName = "pg-foobar-rw.someteamname.svc.cluster.local",
				tunnel = { endpoint = "1.2.3.4:12345", gatewayPublicKey = "gw-public-key" },
			},
		},
	}
end)

Test.gql("PostgresAccess connection rejects a different team member", function(t)
	t.addHeader("x-user-email", otherMemberUser:email())
	t.query [[
		query { postgresAccessConnection(input: {name: "ready-access", teamSlug: "someteamname", environmentName: "dev"}) { password } }
	]]
	t.check {
		errors = { { locations = NotNull(), path = { "postgresAccessConnection" }, message = Contains("not authorized") } },
		data = Null,
	}
end)

Test.gql("PostgresAccess connection rejects expired, unready, and missing-secret access", function(t)
	t.addHeader("x-user-email", user:email())
	for _, test in ipairs({
		{ name = "expired-access",        message = "has expired" },
		{ name = "pending-access",        message = "is not ready" },
		{ name = "missing-secret-access", message = "credentials" },
	}) do
		t.query(string.format(
			[[query { postgresAccessConnection(input: {name: "%s", teamSlug: "someteamname", environmentName: "dev"}) { password } }]],
			test.name))
		t.check {
			errors = { { locations = NotNull(), path = { "postgresAccessConnection" }, message = Contains(test.message) } },
			data = Null,
		}
	end
end)

Test.gql("Personal postgres connection retrieval is audited", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		query { postgresAccessConnection(input: {name: "ready-access", teamSlug: "someteamname", environmentName: "dev"}) { password } }
	]]
	t.check { data = { postgresAccessConnection = { password = "supersecret" } } }

	t.query [[
		{ team(slug: "someteamname") { activityLog(first: 1) { nodes { message ... on PostgresPersonalAccessConnectionActivityLogEntry { resourceName } } } } }
	]]
	t.check {
		data = { team = { activityLog = { nodes = {
			{ message = Contains("Retrieved personal Postgres connection materials"), resourceName = "ready-access" },
		} } } },
	}
end)
