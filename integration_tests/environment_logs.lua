local user = User.new("user-1", "usr@ex.com", "ei")

local function logsQuery(query, from, to)
	return string.format(
		[[
		query {
			environment(name: "dev") {
				logs(filter: {query: %q, from: %q, to: %q}) {
					nodes {
						time
						message
					}
					pageInfo {
						hasNextPage
						endCursor
					}
				}
			}
		}
	]],
		query,
		from,
		to
	)
end

Test.gql("Environment logs - selecting another environment is rejected", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(logsQuery('{service_name="app", k8s_cluster_name="prod"}', "2024-01-01T00:00:00Z", "2024-01-01T01:00:00Z"))

	t.check {
		data = Null,
		errors = {
			{
				message = Contains("other environments"),
				extensions = NotNull(),
				path = { "environment", "logs" },
			},
		},
	}
end)

Test.gql("Environment logs - regex matcher on cluster is rejected", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(logsQuery('{service_name="app", k8s_cluster_name=~".+"}', "2024-01-01T00:00:00Z", "2024-01-01T01:00:00Z"))

	t.check {
		data = Null,
		errors = {
			{
				message = Contains("other environments"),
				extensions = NotNull(),
				path = { "environment", "logs" },
			},
		},
	}
end)

Test.gql("Environment logs - metric queries are rejected", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(logsQuery('count_over_time({service_name="app"}[5m])', "2024-01-01T00:00:00Z", "2024-01-01T01:00:00Z"))

	t.check {
		data = Null,
		errors = {
			{
				message = Contains("Only log queries are supported"),
				extensions = NotNull(),
				path = { "environment", "logs" },
			},
		},
	}
end)

Test.gql("Environment logs - from must be before to", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(logsQuery('{service_name="app"}', "2024-01-01T01:00:00Z", "2024-01-01T01:00:00Z"))

	t.check {
		data = Null,
		errors = {
			{
				message = NotNull(),
				extensions = NotNull(),
				path = { "environment", "logs" },
			},
		},
	}
end)

Test.gql("Environment logs - requires authentication", function(t)
	t.query(logsQuery('{service_name="app"}', "2024-01-01T00:00:00Z", "2024-01-01T01:00:00Z"))

	t.check {
		errors = {
			{
				message = NotNull(),
			},
		},
	}
end)
