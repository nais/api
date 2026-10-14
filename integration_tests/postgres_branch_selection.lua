local user = User.new("pg-selection-user", "pg-selection-user@usersen.com")
local team = Team.new("pg-selection-team", "Testing explicit Postgres branches", "#postgres")
team:addMember(user)
Helper.readK8sResources("k8s_resources/postgres_branch_selection")

Test.gql("Branch workloads distinguish explicit selection from observed active branch", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug: "pg-selection-team") { environment(name: "dev") {
  postgres(name: "orders") {
   main: branch(name: "main") { workloads { nodes { __typename name } } }
   active: branch(name: "green") { workloads { nodes { __typename name } } }
   preview: branch(name: "pr-123") { workloads { nodes { __typename name } } }
   requested: branch(name: "pending") { workloads { nodes { __typename name } } }
  }
 } } }]]
	t.check { data = { team = { environment = { postgres = {
		main = { workloads = { nodes = { { __typename = "Application", name = "pinned-app" } } } },
		active = { workloads = { nodes = { { __typename = "Application", name = "following-app" } } } },
		preview = { workloads = { nodes = { { __typename = "Job", name = "pinned-job" } } } },
		requested = { workloads = { nodes = {} } },
	} } } } }
end)

Test.gql("Workload Postgres usages resolve explicit, active, and pending selections", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug: "pg-selection-team") { environment(name: "dev") {
		pinned: application(name: "pinned-app") { uses { nodes { ... on PostgresUsage { postgres { name } branch { name } } } } }
		following: application(name: "following-app") { uses { nodes { ... on PostgresUsage { postgres { name } branch { name } } } } }
		job: job(name: "pinned-job") { uses { nodes { ... on PostgresUsage { postgres { name } branch { name } } } } }
	} } }]]
	t.check { data = { team = { environment = {
		pinned = { uses = { nodes = { { postgres = { name = "orders" }, branch = { name = "main" } } } } },
		following = { uses = { nodes = { { postgres = { name = "orders" }, branch = { name = "green" } } } } },
		job = { uses = { nodes = { { postgres = { name = "orders" }, branch = { name = "pr-123" } } } } },
	} } } }
end)

Test.gql("Postgres workload branches use explicit selection or the observed active branch", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug: "pg-selection-team") { environment(name: "dev") {
		postgres(name: "orders") {
			desiredActiveBranch activeBranch { name }
			workloads { pageInfo { totalCount } nodes {
				workload {
					... on Application { __typename name }
					... on Job { __typename name }
				}
				branch { name }
			} }
		}
	} } }]]
	t.check { data = { team = { environment = { postgres = {
		desiredActiveBranch = "pending", activeBranch = { name = "green" },
		workloads = { pageInfo = { totalCount = 3 }, nodes = {
			{ workload = { __typename = "Application", name = "following-app" }, branch = { name = "green" } },
			{ workload = { __typename = "Application", name = "pinned-app" },    branch = { name = "main" } },
			{ workload = { __typename = "Job", name = "pinned-job" },            branch = { name = "pr-123" } },
		} },
	} } } } }
end)

for _, test in ipairs({
	{ branch = "main",    reference = 'Application "pinned-app"' },
	{ branch = "pr-123",  reference = 'Naisjob "pinned-job"' },
	{ branch = "binding", reference = 'PostgresBinding "standalone-binding"' },
}) do
	Test.gql("Deleting a branch referenced by " .. test.reference .. " is allowed and audited", function(t)
		t.addHeader("x-user-email", user:email())
		t.query([[mutation { deletePostgresBranch(input: { postgres: "orders", branch: "]] ..
			test.branch .. [[", environmentName: "dev", teamSlug: "pg-selection-team" }) { postgresBranchDeleted } }]])
		t.check { data = { deletePostgresBranch = { postgresBranchDeleted = true } } }
		t.query [[{ team(slug: "pg-selection-team") {
			activityLog(first: 1, filter: { activityTypes: [POSTGRES_BRANCH_DELETED] }) {
				nodes { actor createdAt resourceName teamSlug environmentName
					... on PostgresBranchDeletedActivityLogEntry { data { branch } }
				}
			}
		} }]]
		t.check { data = { team = { activityLog = { nodes = {
			{ actor = user:email(), createdAt = NotNull(), resourceName = "orders", teamSlug = "pg-selection-team", environmentName = "dev", data = { branch = test.branch } },
		} } } } }
	end)
end

Test.gql("A missing referenced branch returns an error instead of omitting the workload", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug: "pg-selection-team") { environment(name: "dev") { postgres(name: "orders") {
		workloads { pageInfo { totalCount } nodes { workload {
			... on Application { __typename name }
			... on Job { __typename name }
		} } }
	} } } }]]
	t.check { data = { team = { environment = { postgres = { workloads = {
		pageInfo = { totalCount = 3 }, nodes = {
		{ workload = { __typename = "Application", name = "following-app" } },
		{ workload = { __typename = "Application", name = "pinned-app" } },
		{ workload = { __typename = "Job", name = "pinned-job" } },
	},
	} } } } } }
	t.query [[{ team(slug: "pg-selection-team") { environment(name: "dev") { postgres(name: "orders") {
		workloads(first: 2) { nodes { workload { ... on Application { name } } branch { name } } }
	} } } }]]
	t.check {
		data = Null,
		errors = { { locations = NotNull(), message = Contains("orders/main"), path = { "team", "environment", "postgres", "workloads", "nodes", 1, "branch" } } },
	}
	t.query [[{ team(slug: "pg-selection-team") { environment(name: "dev") {
		application(name: "pinned-app") { uses { nodes { ... on PostgresUsage { postgres { name } branch { name } } } } }
	} } }]]
	t.check {
		data = Null,
		errors = { { locations = NotNull(), message = Contains("orders/main"), path = { "team", "environment", "application", "uses" } } },
	}
end)

Test.gql("Pending activation also prevents deletion", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[mutation { deletePostgresBranch(input: { postgres: "orders", branch: "pending", environmentName: "dev", teamSlug: "pg-selection-team" }) { postgresBranchDeleted } }]]
	t.check { errors = { { locations = NotNull(), path = { "deletePostgresBranch" }, message = Contains("is active and cannot be deleted") } }, data = Null }
end)

Test.gql("References to other branches do not block deleting an unused branch", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[mutation { deletePostgresBranch(input: { postgres: "orders", branch: "old", environmentName: "dev", teamSlug: "pg-selection-team" }) { postgresBranchDeleted } }]]
	t.check { data = { deletePostgresBranch = { postgresBranchDeleted = true } } }
end)
