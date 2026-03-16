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

-- Input validation: TTL exceeds maximum
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

-- Input validation: invalid TTL format
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

-- Input validation: zero TTL
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
