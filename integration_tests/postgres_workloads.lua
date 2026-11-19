local user = User.new("pg-workload-user", "pg-workload-user@usersen.com")
local team = Team.new("postgres-workload-team", "Testing workload Postgres uses", "#postgres-workloads")
team:addMember(user)
Helper.readK8sResources("k8s_resources/postgres_workloads")

local workloadFields = [[fragment PostgresWorkloadFields on PostgresWorkload {
	workload {
		... on Application { __typename id name team { slug } teamEnvironment { environment { name } } }
		... on Job { __typename id name team { slug } teamEnvironment { environment { name } } }
	}
}]]

local function workloadNode(workloadType, name, branch)
	return {
		workload = {
			__typename = workloadType,
			id = NotNull(),
			name = name,
			team = { slug = "postgres-workload-team" },
			teamEnvironment = { environment = { name = "dev" } },
		},
	}
end

Test.gql("Application and job Postgres uses expose spec fields and resolved resources", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug: "postgres-workload-team") { environment(name: "dev") {
		application(name: "consumer") { postgreses { pageInfo { totalCount } nodes { envPrefix role postgres { name } } } }
		job(name: "scheduled-reader") { postgreses { pageInfo { totalCount } nodes { envPrefix role postgres { name } } } }
	} } }]]
	local applicationPostgreses = {
		{ envPrefix = "ARCHIVE_", role = "ADMIN", postgres = { name = "archive" } },
		{ envPrefix = Null,       role = "READ",  postgres = { name = "orders" } },
		{ envPrefix = "REPORTS_", role = "ADMIN", postgres = { name = "reports" } },
	}
	local jobPostgreses = {
		{ envPrefix = "ARCHIVE_", role = "ADMIN", postgres = { name = "archive" } },
		{ envPrefix = Null,       role = "ADMIN", postgres = { name = "orders" } },
		{ envPrefix = "REPORTS_", role = "ADMIN", postgres = { name = "reports" } },
	}
	t.check { data = { team = { environment = {
		application = { postgreses = { pageInfo = { totalCount = 3 }, nodes = applicationPostgreses } },
		job = { postgreses = { pageInfo = { totalCount = 3 }, nodes = jobPostgreses } },
	} } } }
end)

Test.gql("Postgres workloads reference their database through uses.postgres", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug: "postgres-workload-team") { environment(name: "dev") {
		postgres(name: "orders") { workloads { nodes { workload { __typename ... on Application { name } ... on Job { name } } } } }
		other: postgres(name: "reports") { workloads { nodes { workload { __typename ... on Application { name } ... on Job { name } } } } }
	} } }]]
	local workloads = {
		{ workload = { __typename = "Application", name = "consumer" } },
		{ workload = { __typename = "Job", name = "scheduled-reader" } },
	}
	t.check { data = { team = { environment = {
		postgres = { workloads = { nodes = workloads } },
		other = { workloads = { nodes = workloads } },
	} } } }
end)

Test.gql("Postgres workloads include inactive branch references and databases without an active branch", function(t)
	t.addHeader("x-user-email", user:email())
	t.query([[{ team(slug: "postgres-workload-team") { environment(name: "dev") {
		postgres(name: "orders") { workloads(first: 1) { pageInfo { totalCount hasNextPage endCursor } nodes { ...PostgresWorkloadFields } } }
		reports: postgres(name: "reports") { workloads { pageInfo { totalCount } nodes { ...PostgresWorkloadFields } } }
		archive: postgres(name: "archive") { workloads { pageInfo { totalCount } nodes { ...PostgresWorkloadFields } } }
	} } }]] .. workloadFields)
	t.check { data = { team = { environment = {
		postgres = { workloads = { pageInfo = { totalCount = 2, hasNextPage = true, endCursor = Save("ordersCursor") }, nodes = {
			workloadNode("Application", "consumer", "main"),
		} } },
		reports = { workloads = { pageInfo = { totalCount = 2 }, nodes = {
			workloadNode("Application", "consumer", "main"),
			workloadNode("Job", "scheduled-reader", "main"),
		} } },
		archive = { workloads = { pageInfo = { totalCount = 2 }, nodes = {
			workloadNode("Application", "consumer", "main"),
			workloadNode("Job", "scheduled-reader", "main"),
		} } },
	} } } }
	t.query([[{ team(slug: "postgres-workload-team") { environment(name: "dev") {
		postgres(name: "orders") { workloads(first: 1, after: "]] .. State["ordersCursor"] .. [[") {
			pageInfo { totalCount hasNextPage hasPreviousPage }
			nodes { ...PostgresWorkloadFields }
		} }
	} } }]] .. workloadFields)
	t.check { data = { team = { environment = { postgres = { workloads = {
		pageInfo = { totalCount = 2, hasNextPage = false, hasPreviousPage = true },
		nodes = { workloadNode("Job", "scheduled-reader", "main") },
	} } } } } }
end)
