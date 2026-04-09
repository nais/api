local user = User.new("user", "user@usersen.com")
local nonMember = User.new("nonmember", "other@user.com")

local team = Team.new("credteam", "purpose", "#slack_channel")
team:addMember(user)

-- Load K8s fixtures so that instance existence checks pass for known instances.
-- The fixtures provide opensearch-credteam-my-opensearch and valkey-credteam-my-valkey in the "dev" cluster.
Helper.readK8sResources("k8s_resources/aiven_credentials")

-- Authorization: non-member cannot create OpenSearch credentials
Test.gql("Non-member cannot create OpenSearch credentials", function(t)
	t.addHeader("x-user-email", nonMember:email())
	t.query(string.format([[
		mutation {
		  createOpenSearchCredentials(input: {
		    teamSlug: "%s"
		    environmentName: "dev"
		    instanceName: "my-opensearch"
		    permission: READ
		    ttl: "1d"
		  }) {
		    credentials { username }
		  }
		}
	]], team:slug()))

	t.check {
		errors = {
			{
				message = Contains("you need the \"aiven:credentials:create\" authorization"),
				path = { "createOpenSearchCredentials" },
			},
		},
		data = Null,
	}
end)

-- Authorization: non-member cannot create Valkey credentials
Test.gql("Non-member cannot create Valkey credentials", function(t)
	t.addHeader("x-user-email", nonMember:email())
	t.query(string.format([[
		mutation {
		  createValkeyCredentials(input: {
		    teamSlug: "%s"
		    environmentName: "dev"
		    instanceName: "my-valkey"
		    permission: READ
		    ttl: "1d"
		  }) {
		    credentials { username }
		  }
		}
	]], team:slug()))

	t.check {
		errors = {
			{
				message = Contains("you need the \"aiven:credentials:create\" authorization"),
				path = { "createValkeyCredentials" },
			},
		},
		data = Null,
	}
end)

-- Authorization: non-member cannot create Kafka credentials
Test.gql("Non-member cannot create Kafka credentials", function(t)
	t.addHeader("x-user-email", nonMember:email())
	t.query(string.format([[
		mutation {
		  createKafkaCredentials(input: {
		    teamSlug: "%s"
		    environmentName: "dev"
		    ttl: "1d"
		  }) {
		    credentials { username }
		  }
		}
	]], team:slug()))

	t.check {
		errors = {
			{
				message = Contains("you need the \"aiven:credentials:create\" authorization"),
				path = { "createKafkaCredentials" },
			},
		},
		data = Null,
	}
end)

-- Authorization: non-existing team
Test.gql("Cannot create credentials for non-existing team", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation {
		  createOpenSearchCredentials(input: {
		    teamSlug: "does-not-exist"
		    environmentName: "dev"
		    instanceName: "my-opensearch"
		    permission: READ
		    ttl: "1d"
		  }) {
		    credentials { username }
		  }
		}
	]]

	t.check {
		errors = {
			{
				message = Contains("you need the \"aiven:credentials:create\" authorization"),
				path = { "createOpenSearchCredentials" },
			},
		},
		data = Null,
	}
end)

-- Instance not found: OpenSearch
Test.gql("Cannot create OpenSearch credentials for non-existing instance", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		mutation {
		  createOpenSearchCredentials(input: {
		    teamSlug: "%s"
		    environmentName: "dev"
		    instanceName: "does-not-exist"
		    permission: READ
		    ttl: "1d"
		  }) {
		    credentials { username }
		  }
		}
	]], team:slug()))

	t.check {
		errors = {
			{
				message = Contains("not found"),
				path = { "createOpenSearchCredentials" },
			},
		},
		data = Null,
	}
end)

-- Instance not found: Valkey
Test.gql("Cannot create Valkey credentials for non-existing instance", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		mutation {
		  createValkeyCredentials(input: {
		    teamSlug: "%s"
		    environmentName: "dev"
		    instanceName: "does-not-exist"
		    permission: READ
		    ttl: "1d"
		  }) {
		    credentials { username }
		  }
		}
	]], team:slug()))

	t.check {
		errors = {
			{
				message = Contains("not found"),
				path = { "createValkeyCredentials" },
			},
		},
		data = Null,
	}
end)

-- Instance not found: non-existing environment for OpenSearch
Test.gql("Cannot create OpenSearch credentials in non-existing environment", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		mutation {
		  createOpenSearchCredentials(input: {
		    teamSlug: "%s"
		    environmentName: "prod"
		    instanceName: "my-opensearch"
		    permission: READ
		    ttl: "1d"
		  }) {
		    credentials { username }
		  }
		}
	]], team:slug()))

	t.check {
		errors = {
			{
				message = Contains("not found"),
				path = { "createOpenSearchCredentials" },
			},
		},
		data = Null,
	}
end)

-- Instance not found: non-existing environment for Valkey
Test.gql("Cannot create Valkey credentials in non-existing environment", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		mutation {
		  createValkeyCredentials(input: {
		    teamSlug: "%s"
		    environmentName: "prod"
		    instanceName: "my-valkey"
		    permission: READ
		    ttl: "1d"
		  }) {
		    credentials { username }
		  }
		}
	]], team:slug()))

	t.check {
		errors = {
			{
				message = Contains("not found"),
				path = { "createValkeyCredentials" },
			},
		},
		data = Null,
	}
end)

-- Input validation: TTL exceeds maximum (OpenSearch)
Test.gql("TTL exceeding 30 days is rejected", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		mutation {
		  createOpenSearchCredentials(input: {
		    teamSlug: "%s"
		    environmentName: "dev"
		    instanceName: "my-opensearch"
		    permission: READ
		    ttl: "31d"
		  }) {
		    credentials { username }
		  }
		}
	]], team:slug()))

	t.check {
		errors = {
			{
				message = Contains("TTL exceeds maximum of 30 days"),
				path = { "createOpenSearchCredentials" },
			},
		},
		data = Null,
	}
end)

-- Input validation: TTL exceeds maximum (Valkey)
Test.gql("Valkey TTL exceeding 30 days is rejected", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		mutation {
		  createValkeyCredentials(input: {
		    teamSlug: "%s"
		    environmentName: "dev"
		    instanceName: "my-valkey"
		    permission: READ
		    ttl: "31d"
		  }) {
		    credentials { username }
		  }
		}
	]], team:slug()))

	t.check {
		errors = {
			{
				message = Contains("TTL exceeds maximum of 30 days"),
				path = { "createValkeyCredentials" },
			},
		},
		data = Null,
	}
end)

-- Input validation: TTL exceeds maximum (Kafka)
Test.gql("Kafka TTL exceeding 365 days is rejected", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		mutation {
		  createKafkaCredentials(input: {
		    teamSlug: "%s"
		    environmentName: "dev"
		    ttl: "366d"
		  }) {
		    credentials { username }
		  }
		}
	]], team:slug()))

	t.check {
		errors = {
			{
				message = Contains("TTL exceeds maximum of 365 days"),
				path = { "createKafkaCredentials" },
			},
		},
		data = Null,
	}
end)

-- Input validation: invalid TTL format (Valkey)
Test.gql("Invalid TTL format is rejected", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		mutation {
		  createValkeyCredentials(input: {
		    teamSlug: "%s"
		    environmentName: "dev"
		    instanceName: "my-valkey"
		    permission: READWRITE
		    ttl: "abc"
		  }) {
		    credentials { username }
		  }
		}
	]], team:slug()))

	t.check {
		errors = {
			{
				message = Contains("invalid TTL"),
				path = { "createValkeyCredentials" },
			},
		},
		data = Null,
	}
end)

-- Input validation: invalid TTL format (OpenSearch)
Test.gql("OpenSearch invalid TTL format is rejected", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		mutation {
		  createOpenSearchCredentials(input: {
		    teamSlug: "%s"
		    environmentName: "dev"
		    instanceName: "my-opensearch"
		    permission: READ
		    ttl: "abc"
		  }) {
		    credentials { username }
		  }
		}
	]], team:slug()))

	t.check {
		errors = {
			{
				message = Contains("invalid TTL"),
				path = { "createOpenSearchCredentials" },
			},
		},
		data = Null,
	}
end)

-- Input validation: invalid TTL format (Kafka)
Test.gql("Kafka invalid TTL format is rejected", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		mutation {
		  createKafkaCredentials(input: {
		    teamSlug: "%s"
		    environmentName: "dev"
		    ttl: "abc"
		  }) {
		    credentials { username }
		  }
		}
	]], team:slug()))

	t.check {
		errors = {
			{
				message = Contains("invalid TTL"),
				path = { "createKafkaCredentials" },
			},
		},
		data = Null,
	}
end)

-- Input validation: zero TTL (Kafka)
Test.gql("Zero TTL is rejected", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		mutation {
		  createKafkaCredentials(input: {
		    teamSlug: "%s"
		    environmentName: "dev"
		    ttl: "0d"
		  }) {
		    credentials { username }
		  }
		}
	]], team:slug()))

	t.check {
		errors = {
			{
				message = Contains("TTL must be positive"),
				path = { "createKafkaCredentials" },
			},
		},
		data = Null,
	}
end)

-- Input validation: zero TTL (OpenSearch)
Test.gql("OpenSearch zero TTL is rejected", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		mutation {
		  createOpenSearchCredentials(input: {
		    teamSlug: "%s"
		    environmentName: "dev"
		    instanceName: "my-opensearch"
		    permission: READ
		    ttl: "0d"
		  }) {
		    credentials { username }
		  }
		}
	]], team:slug()))

	t.check {
		errors = {
			{
				message = Contains("TTL must be positive"),
				path = { "createOpenSearchCredentials" },
			},
		},
		data = Null,
	}
end)

-- Input validation: zero TTL (Valkey)
Test.gql("Valkey zero TTL is rejected", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		mutation {
		  createValkeyCredentials(input: {
		    teamSlug: "%s"
		    environmentName: "dev"
		    instanceName: "my-valkey"
		    permission: READ
		    ttl: "0d"
		  }) {
		    credentials { username }
		  }
		}
	]], team:slug()))

	t.check {
		errors = {
			{
				message = Contains("TTL must be positive"),
				path = { "createValkeyCredentials" },
			},
		},
		data = Null,
	}
end)

-- Insert activity log entries directly via SQL (we can't run the full credential
-- creation flow in integration tests since there's no Aivenator), then verify
-- that the GraphQL activity log query returns them correctly. This is the test
-- that would have panicked before adding the GraphQL schema types.
Helper.SQLExec(string.format([[
	INSERT INTO activity_log_entries (actor, action, resource_type, resource_name, team_slug, environment, data, created_at)
	VALUES
		('%s', 'CREATE_CREDENTIALS', 'CREDENTIALS', 'OPENSEARCH', '%s', 'dev', '{"serviceType":"OPENSEARCH","instanceName":"my-instance","permission":"READ","ttl":"1d"}', NOW() - INTERVAL '2 minutes'),
		('%s', 'CREATE_CREDENTIALS', 'CREDENTIALS', 'KAFKA', '%s', 'dev', '{"serviceType":"KAFKA","ttl":"7d"}', NOW() - INTERVAL '1 minute')
]], user:email(), team:slug(), user:email(), team:slug()))

Test.gql("Activity log returns credentials entries without panic", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		{
		  team(slug: "%s") {
		    activityLog(first: 10, filter: { activityTypes: [CREDENTIALS_CREATE] }) {
		      nodes {
		        __typename
		        message
		        actor
		        resourceType
		        resourceName
		        environmentName
		        ... on CredentialsActivityLogEntry {
		          data {
		            serviceType
		            instanceName
		            permission
		            ttl
		          }
		        }
		      }
		    }
		  }
		}
	]], team:slug()))

	t.check {
		data = {
			team = {
				activityLog = {
					nodes = {
						{
							__typename = "CredentialsActivityLogEntry",
							message = "Created KAFKA credentials (TTL: 7d)",
							actor = user:email(),
							resourceType = "CREDENTIALS",
							resourceName = "KAFKA",
							environmentName = "dev",
							data = {
								serviceType = "KAFKA",
								instanceName = "",
								permission = "",
								ttl = "7d",
							},
						},
						{
							__typename = "CredentialsActivityLogEntry",
							message = "Created OPENSEARCH credentials for my-instance with READ permission (TTL: 1d)",
							actor = user:email(),
							resourceType = "CREDENTIALS",
							resourceName = "OPENSEARCH",
							environmentName = "dev",
							data = {
								serviceType = "OPENSEARCH",
								instanceName = "my-instance",
								permission = "READ",
								ttl = "1d",
							},
						},
					},
				},
			},
		},
	}
end)
