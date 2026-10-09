local member = User.new("pg-search-user", "pg-search-user@usersen.com")
local team = Team.new("pg-search-team", "Postgres search", "#pg-search")
local other = Team.new("pg-search-other", "Other Postgres search team", "#pg-search-other")
team:addMember(member)
other:addMember(member)
Helper.readK8sResources("k8s_resources/postgres_search")

Test.gql("Search returns logical Postgres databases rather than branches", function(t)
	t.addHeader("x-user-email", member:email())
	t.query [[{
		search(first: 10, filter: { query: "orders", types: [POSTGRES], teams: ["pg-search-team"] }) {
			nodes {
				__typename
				... on Postgres {
					id
					name
					team { slug }
					teamEnvironment { environment { name } }
				}
			}
		}
	}]]
	t.check {
		data = {
			search = {
				nodes = {
					{
						__typename = "Postgres",
						id = NotNull(),
						name = "orders",
						team = { slug = "pg-search-team" },
						teamEnvironment = { environment = { name = "dev" } },
					},
				},
			},
		},
	}
end)

Test.gql("Postgres search scopes matching names to the requested team", function(t)
	t.addHeader("x-user-email", member:email())
	t.query [[{
		search(first: 10, filter: { query: "orders", types: [POSTGRES], teams: ["pg-search-other"] }) {
			nodes {
				... on Postgres { name majorVersion team { slug } }
			}
		}
	}]]
	t.check {
		data = {
			search = {
				nodes = {
					{ name = "orders", majorVersion = "18", team = { slug = "pg-search-other" } },
				},
			},
		},
	}
end)

Test.gql("Postgres search does not return branch names", function(t)
	t.addHeader("x-user-email", member:email())
	t.query [[{
		search(first: 10, filter: { query: "green", types: [POSTGRES], teams: ["pg-search-team"] }) {
			nodes { __typename }
		}
	}]]
	t.check { data = { search = { nodes = {} } } }
end)
