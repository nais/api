User.new("Tenant user", "tenant@example.com", "tenant", "TENANT")
local nais = User.new("Nais user", "viewer@example.com", "nais", "NAIS")
User.new("Unclassified user", "unclassified@nais.io", "unclassified")

Test.gql("expose the recorded user source", function(t)
	t.addHeader("x-user-email", nais:email())
	t.query [[
		query {
			me { ... on User { source } }
			tenant: user(email: "tenant@example.com") { source }
			unclassified: user(email: "unclassified@nais.io") { source }
		}
	]]
	t.check {
		data = {
			me = { source = "NAIS" },
			tenant = { source = "TENANT" },
			unclassified = { source = Null },
		},
	}
end)

Test.gql("filter the users list by source", function(t)
	t.addHeader("x-user-email", nais:email())
	t.query [[
		query {
			nais: users(first: 1, filter: {sources: [NAIS]}) {
				nodes { source }
				pageInfo { totalCount hasNextPage }
			}
			tenant: users(filter: {sources: [TENANT]}) {
				nodes { source }
				pageInfo { totalCount }
			}
			both: users(filter: {sources: [TENANT, NAIS]}) { pageInfo { totalCount } }
			neither: users(filter: {sources: []}) { pageInfo { totalCount } }
			unfiltered: users { pageInfo { totalCount } }
		}
	]]
	t.check {
		data = {
			nais = { nodes = { { source = "NAIS" } }, pageInfo = { totalCount = 1, hasNextPage = false } },
			tenant = { nodes = { { source = "TENANT" } }, pageInfo = { totalCount = 1 } },
			both = { pageInfo = { totalCount = 2 } },
			neither = { pageInfo = { totalCount = 0 } },
			unfiltered = { pageInfo = { totalCount = 3 } },
		},
	}
end)
