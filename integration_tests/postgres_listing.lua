local member = User.new("pg-branch-user", "pg-branch-user@usersen.com")
local team = Team.new("pg-crud-team", "Testing Postgres listing", "#postgres-crud")
team:addMember(member)
Helper.readK8sResources("k8s_resources/postgres_crud")

Test.gql("Team Postgres list includes databases without branches and paginates", function(t)
	t.addHeader("x-user-email", member:email())
	t.query [[{ team(slug: "pg-crud-team") { postgreses(first: 1) {
		nodes { name majorVersion teamEnvironment { environment { name } } }
		pageInfo { totalCount hasNextPage endCursor }
		facets { labels { key value count } }
	} } }]]
	t.check { data = { team = { postgreses = {
		nodes = { { name = "branchless", majorVersion = "17", teamEnvironment = { environment = { name = "dev" } } } },
		pageInfo = { totalCount = 2, hasNextPage = true, endCursor = Save("postgresNextPage") },
		facets = { labels = { { key = "usecase", value = "reporting", count = 1 } } },
	} } } }
	t.query(string.format([[{ team(slug: "pg-crud-team") { postgreses(first: 1, after: "%s") {
		nodes { name } pageInfo { totalCount hasNextPage }
	} } }]], State.postgresNextPage))
	t.check { data = { team = { postgreses = {
		nodes = { { name = "existing" } }, pageInfo = { totalCount = 2, hasNextPage = false },
	} } } }
end)

Test.gql("Team Postgres list filters instances by label and environment", function(t)
	t.addHeader("x-user-email", member:email())
	t.query [[{ team(slug: "pg-crud-team") { postgreses(filter: {
		environments: ["dev"], labels: [{ key: "usecase", value: "reporting" }]
	}) { nodes { name } pageInfo { totalCount } } } }]]
	t.check { data = { team = { postgreses = { nodes = { { name = "branchless" } }, pageInfo = { totalCount = 1 } } } } }
end)
