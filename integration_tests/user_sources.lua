local tenant = User.new("Tenant user", "tenant@example.com", "tenant", "TENANT")
tenant:admin(true)
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

Test.gql("filter the users list by global admin status", function(t)
	t.addHeader("x-user-email", nais:email())
	t.query [[
		query {
			admins: users(first: 1, filter: {isAdmin: true}) {
				nodes { email isAdmin }
				pageInfo { totalCount hasNextPage }
			}
			nonAdmins: users(filter: {isAdmin: false}) {
				nodes { email isAdmin }
				pageInfo { totalCount }
			}
			tenantAdmins: users(filter: {sources: [TENANT], isAdmin: true}) { pageInfo { totalCount } }
			naisAdmins: users(filter: {sources: [NAIS], isAdmin: true}) { pageInfo { totalCount } }
		}
	]]
	t.check {
		data = {
			admins = {
				nodes = { { email = "tenant@example.com", isAdmin = true } },
				pageInfo = { totalCount = 1, hasNextPage = false },
			},
			nonAdmins = {
				nodes = {
					{ email = "viewer@example.com",   isAdmin = false },
					{ email = "unclassified@nais.io", isAdmin = false },
				},
				pageInfo = { totalCount = 2 },
			},
			tenantAdmins = { pageInfo = { totalCount = 1 } },
			naisAdmins = { pageInfo = { totalCount = 0 } },
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
