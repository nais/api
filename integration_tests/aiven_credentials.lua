local user = User.new("user", "user@usersen.com")
local nonMember = User.new("nonmember", "other@user.com")

local team = Team.new("credteam", "purpose", "#slack_channel")
team:addMember(user)

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
				locations = NotNull(),
				message = Contains("you need the \"opensearch:credentials:create\" authorization"),
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
				locations = NotNull(),
				message = Contains("you need the \"valkey:credentials:create\" authorization"),
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
				locations = NotNull(),
				message = Contains("you need the \"kafka:credentials:create\" authorization"),
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
				locations = NotNull(),
				message = Contains("you need the \"opensearch:credentials:create\" authorization"),
				path = { "createOpenSearchCredentials" },
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
				locations = NotNull(),
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
				locations = NotNull(),
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
				locations = NotNull(),
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
				locations = NotNull(),
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
				locations = NotNull(),
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
				locations = NotNull(),
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
				locations = NotNull(),
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
				locations = NotNull(),
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
				locations = NotNull(),
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
		('%s', 'CREDENTIALS_CREATED', 'OPENSEARCH', 'my-instance', '%s', 'dev', '{"permission":"READ","ttl":"1d"}', NOW() - INTERVAL '2 minutes'),
		('%s', 'CREDENTIALS_CREATED', 'VALKEY', 'my-valkey', '%s', 'dev', '{"ttl":"7d"}', NOW() - INTERVAL '1 minute'),
		('%s', 'CREDENTIALS_CREATED', 'KAFKA_TOPIC', 'dev', '%s', 'dev', '{"ttl":"7d"}', NOW() - INTERVAL '5 minute')
]], user:email(), team:slug(), user:email(), team:slug(), user:email(), team:slug()))

Test.gql("Activity log returns credentials entries without panic", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		{
		  team(slug: "%s") {
		    activityLog(first: 10, filter: { activityTypes: [VALKEY_CREDENTIALS_CREATED, OPENSEARCH_CREDENTIALS_CREATED, KAFKA_CREDENTIALS_CREATED] }) {
		      nodes {
		        __typename
		        message
		        actor
		        resourceType
		        resourceName
		        environmentName
		        ... on ValkeyCredentialsCreatedActivityLogEntry {
		          data {
		            permission
		            ttl
		          }
		        }
		        ... on OpenSearchCredentialsCreatedActivityLogEntry {
		          data {
		            permission
		            ttl
		          }
		        }
		        ... on KafkaCredentialsCreatedActivityLogEntry {
		          data {
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
							__typename = "ValkeyCredentialsCreatedActivityLogEntry",
							message = "Created credentials for \"my-valkey\" (TTL: 7d)",
							actor = user:email(),
							resourceType = "VALKEY",
							resourceName = "my-valkey",
							environmentName = "dev",
							data = {
								permission = "",
								ttl = "7d",
							},
						},
						{
							__typename = "OpenSearchCredentialsCreatedActivityLogEntry",
							message = "Created credentials for \"my-instance\" with \"read\" permission (TTL: 1d)",
							actor = user:email(),
							resourceType = "OPENSEARCH",
							resourceName = "my-instance",
							environmentName = "dev",
							data = {
								permission = "READ",
								ttl = "1d",
							},
						},
						{
							__typename = "KafkaCredentialsCreatedActivityLogEntry",
							message = "Created credentials for \"dev\" (TTL: 7d)",
							actor = user:email(),
							resourceType = "KAFKA_TOPIC",
							resourceName = "dev",
							environmentName = "dev",
							data = {
								ttl = "7d",
							},
						},
					},
				},
			},
		},
	}
end)

Helper.readK8sResources("k8s_resources/kafka_credentials")

for _, environment in ipairs({ "dev-gcp", "legacy-dev" }) do
	Test.gql("Kafka credentials resolve " .. environment .. " to the canonical environment", function(t)
		t.addHeader("x-user-email", user:email())
		t.query(string.format([[
			mutation {
				createKafkaCredentials(input: {
					teamSlug: "%s", environmentName: "%s", ttl: "1d"
				}) {
					environmentName
					credentials { username accessCert accessKey caCert brokers schemaRegistry }
				}
			}
		]], team:slug(), environment))
		t.check { data = { createKafkaCredentials = {
			environmentName = "dev-gcp",
			credentials = {
				username = "test", accessCert = "test", accessKey = "test",
				caCert = "test", brokers = "test", schemaRegistry = "test",
			},
		} } }
	end)
end

Test.k8s("Kafka credentials use the configured pool in the canonical environment", function(t)
	t.check("aiven.nais.io/v1", "aivenapplications", "dev-gcp", team:slug(), "tmp-kafka-topic-011e34", {
		apiVersion = "aiven.nais.io/v1",
		kind = "AivenApplication",
		metadata = Ignore(),
		spec = {
			protected = true,
			expiresAt = NotNull(),
			kafka = { pool = "aiven-dev", secretName = "tmp-kafka-topic-b2fb77" },
		},
	})
end)

Test.gql("Kafka credentials are audited under the canonical requested environment", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		{ team(slug: "%s") {
			activityLog(first: 1, filter: {activityTypes: [KAFKA_CREDENTIALS_CREATED]}) {
				nodes { environmentName resourceName }
			}
		} }
	]], team:slug()))
	t.check { data = { team = { activityLog = { nodes = {
		{ environmentName = "dev-gcp", resourceName = "dev-gcp" },
	} } } } }
end)

Test.gql("A real Nais environment named dev remains canonical", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		mutation { createKafkaCredentials(input: {
			teamSlug: "%s", environmentName: "dev", ttl: "1d"
		}) { environmentName credentials { username } } }
	]], team:slug()))
	t.check { data = { createKafkaCredentials = {
		environmentName = "dev", credentials = { username = "test" },
	} } }
end)

Test.gql("An environment without Kafka credentials returns a useful error", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		mutation { createKafkaCredentials(input: {
			teamSlug: "%s", environmentName: "staging", ttl: "1d"
		}) { credentials { username } } }
	]], team:slug()))
	t.check {
		data = Null,
		errors = { {
			locations = NotNull(),
			message = 'Kafka credentials are not configured for Nais environment "staging"; valid environments: dev, dev-gcp',
			path = { "createKafkaCredentials" },
		} },
	}
end)
