local user = User.new("user", "user@usersen.com")
local team = Team.new("someteamname", "purpose", "#slack_channel")
team:addMember(user)
local otherTeam = Team.new("postgres-other-team", "Isolation", "#postgres")
otherTeam:addMember(user)
Helper.readK8sResources("k8s_resources/postgres_branches")

local workloadFields = [[fragment PostgresWorkloadFields on PostgresWorkload {
	workload {
		... on Application { __typename id name team { slug } teamEnvironment { environment { name } } }
		... on Job { __typename id name team { slug } teamEnvironment { environment { name } } }
	}
}]]

local function workloadNode(workloadType, name, teamSlug, environmentName)
	teamSlug = teamSlug or "someteamname"
	environmentName = environmentName or "dev"
	return {
		workload = {
			__typename = workloadType,
			id = NotNull(),
			name = name,
			team = { slug = teamSlug },
			teamEnvironment = { environment = { name = environmentName } },
		},
	}
end

Test.gql("List Postgres databases with their observed states", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug: "someteamname") { postgreses(filter: { environments: ["dev"] }) { nodes {
        name majorVersion state
    } } } }]]
	t.check { data = { team = { postgreses = { nodes = {
		{ name = "another-db",    majorVersion = "16", state = "AVAILABLE" },
		{ name = "broken",        majorVersion = "17", state = "DEGRADED" },
		{ name = "foobar",        majorVersion = "17", state = "AVAILABLE" },
		{ name = "with-audit",    majorVersion = "16", state = "AVAILABLE" },
		{ name = "without-audit", majorVersion = "15", state = "AVAILABLE" },
	} } } } }
end)

Test.gql("Postgres extensions preserve configured names", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug:"someteamname") { environment(name:"dev") {
		configured: postgres(name:"foobar") { extensions }
		empty: postgres(name:"another-db") { extensions }
	} } }]]
	t.check { data = { team = { environment = {
		configured = { extensions = { "pgaudit", "uuid-ossp", "postgis", "timescaledb" } },
		empty = { extensions = {} },
	} } } }
end)

Test.gql("Retrieve logical Postgres and selected physical instance separately", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug:"someteamname") { environment(name:"dev") {
        postgres(name:"foobar") { name state majorVersion highAvailability resources { cpu memory diskSize } teamEnvironment { name } }
    } } }]]
	t.check { data = { team = { environment = {
		postgres = { name = "foobar", state = "AVAILABLE", majorVersion = "17", highAvailability = false, resources = { cpu = "100m", memory = "2Gi", diskSize = "2Gi" }, teamEnvironment = { name = "dev" } },
	} } } }
end)

Test.gql("An unrecoverable Postgres is degraded through the main facade", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug:"someteamname") { environment(name:"dev") {
		postgres(name:"broken") { name state }
	} } }]]
	t.check { data = { team = { environment = {
		postgres = { name = "broken", state = "DEGRADED" },
	} } } }
end)

Test.gql("A workload Postgres use follows the active physical instance", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug:"someteamname") { environment(name:"dev") {
	        application(name:"app-with-postgres") { postgreses { nodes { postgres { name } } } }
        postgres(name:"foobar") { workloads { nodes { workload { __typename ... on Application { name } ... on Job { name } } } } }
    } } }]]
	t.check { data = { team = { environment = {
		application = { postgreses = { nodes = { { postgres = { name = "foobar" } } } } },
		postgres = { workloads = { nodes = {
			{ workload = { __typename = "Application", name = "app-with-postgres" } },
			{ workload = { __typename = "Application", name = "app-with-postgres-2" } },
			{ workload = { __typename = "Job", name = "job-with-postgres" } },
		} } },
	} } } }
end)

Test.gql("Postgres workloads are isolated by database, team and environment", function(t)
	t.addHeader("x-user-email", user:email())
	t.query([[{
		team(slug: "someteamname") {
			dev: environment(name: "dev") { postgres(name: "foobar") { workloads {
				pageInfo { totalCount } nodes { ...PostgresWorkloadFields }
			} } }
			staging: environment(name: "staging") { postgres(name: "foobar") { workloads {
				pageInfo { totalCount } nodes { ...PostgresWorkloadFields }
			} } }
		}
		other: team(slug: "postgres-other-team") { environment(name: "dev") { postgres(name: "foobar") { workloads {
			pageInfo { totalCount } nodes { ...PostgresWorkloadFields }
		} } } }
	}]] .. workloadFields)
	t.check { data = {
		team = {
			dev = { postgres = { workloads = { pageInfo = { totalCount = 3 }, nodes = {
				workloadNode("Application", "app-with-postgres"),
				workloadNode("Application", "app-with-postgres-2"),
				workloadNode("Job", "job-with-postgres"),
			} } } },
			staging = { postgres = { workloads = { pageInfo = { totalCount = 1 }, nodes = {
				workloadNode("Job", "job-with-postgres", "someteamname", "staging"),
			} } } },
		},
		other = { environment = { postgres = { workloads = { pageInfo = { totalCount = 1 }, nodes = {
			workloadNode("Application", "app-with-postgres", "postgres-other-team"),
		} } } } },
	} }
end)

Test.gql("Postgres without workloads has an empty relationship connection", function(t)
	t.addHeader("x-user-email", user:email())
	t.query([[{ team(slug: "someteamname") { environment(name: "dev") { postgres(name: "without-audit") {
		workloads { pageInfo { totalCount hasNextPage hasPreviousPage startCursor endCursor }
			nodes { ...PostgresWorkloadFields } edges { cursor node { ...PostgresWorkloadFields } }
		}
	} } } }]] .. workloadFields)
	t.check { data = { team = { environment = { postgres = { workloads = {
		pageInfo = { totalCount = 0, hasNextPage = false, hasPreviousPage = false, startCursor = Null, endCursor = Null },
		nodes = {}, edges = {},
	} } } } } }
end)

Test.gql("Postgres workload pagination keeps same-name application and job distinct and ordered", function(t)
	t.addHeader("x-user-email", user:email())
	local function fetch(arguments, pageInfo, nodes)
		t.query([[{ team(slug: "someteamname") { environment(name: "dev") { postgres(name: "another-db") {
			workloads(]] .. arguments .. [[) {
				pageInfo { totalCount hasNextPage hasPreviousPage startCursor endCursor }
				nodes { ...PostgresWorkloadFields } edges { cursor node { ...PostgresWorkloadFields } }
			}
		} } } }]] .. workloadFields)
		local edges = {}
		for _, node in ipairs(nodes) do
			table.insert(edges, { cursor = Save("edgeCursor"), node = node })
		end
		t.check { data = { team = { environment = { postgres = { workloads = {
			pageInfo = pageInfo, nodes = nodes, edges = edges,
		} } } } } }
	end
	local firstPage = {
		totalCount = 2,
		hasNextPage = true,
		hasPreviousPage = false,
		startCursor = Save("firstCursor"),
		endCursor = Save("endCursor"),
	}
	fetch("first: 1", firstPage, { workloadNode("Application", "shared-reader") })
	local firstCursor = State.firstCursor
	assert(firstCursor == State.endCursor and firstCursor == State.edgeCursor)
	fetch("first: 1", firstPage, { workloadNode("Application", "shared-reader") })
	assert(firstCursor == State.firstCursor)
	fetch('first: 1, after: "' .. firstCursor .. '"', {
		totalCount = 2,
		hasNextPage = false,
		hasPreviousPage = true,
		startCursor = Save("lastCursor"),
		endCursor = Save("endCursor"),
	}, { workloadNode("Job", "shared-reader") })
	local lastCursor = State.lastCursor
	assert(lastCursor == State.endCursor and lastCursor == State.edgeCursor)
	fetch('last: 1, before: "' .. lastCursor .. '"', firstPage, { workloadNode("Application", "shared-reader") })
	assert(firstCursor == State.firstCursor)
	fetch('first: 1, after: "' .. lastCursor .. '"', {
		totalCount = 2, hasNextPage = false, hasPreviousPage = true, startCursor = Null, endCursor = Null,
	}, {})
end)

Test.gql("Postgres configuration exposes explicitly configured parameters in name order", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ team(slug: "someteamname") { environment(name: "dev") { postgres(name: "foobar") {
		configuration { parameters { name value } }
	} } } }]]
	t.check { data = { team = { environment = { postgres = {
		configuration = { parameters = {
			{ name = "bgwriter_lru_maxpages",      value = "200" },
			{ name = "checkpoint_timeout",         value = "10min" },
			{ name = "commit_delay",               value = "100" },
			{ name = "commit_siblings",            value = "10" },
			{ name = "effective_cache_size",       value = "1536MB" },
			{ name = "effective_io_concurrency",   value = "200" },
			{ name = "huge_pages",                 value = "off" },
			{ name = "log_min_duration_statement", value = "1000" },
			{ name = "maintenance_work_mem",       value = "256MB" },
			{ name = "max_wal_size",               value = "2GB" },
			{ name = "pg_stat_statements.track",   value = "all" },
			{ name = "pgaudit.log",                value = "read,write,ddl,role" },
			{ name = "pgaudit.log_parameter",      value = "on" },
			{ name = "pgaudit.log_relation",       value = "on" },
			{ name = "random_page_cost",           value = "1.1" },
			{ name = "shared_buffers",             value = "512MB" },
			{ name = "track_io_timing",            value = "on" },
			{ name = "wal_compression",            value = "zstd" },
			{ name = "work_mem",                   value = "32MB" },
		} },
	} } } } }
end)

Test.gql("Postgres configuration is scoped to branch, database, team and environment", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{
		team(slug: "someteamname") {
			dev: environment(name: "dev") {
				postgres(name: "broken") { configuration { parameters { name value } } }
				empty: postgres(name: "another-db") { configuration { parameters { name value } } }
				missing: postgres(name: "without-audit") { name configuration { parameters { name value } } }
			}
			staging: environment(name: "staging") { postgres(name: "foobar") { configuration { parameters { name value } } } }
		}
		other: team(slug: "postgres-other-team") { environment(name: "dev") { postgres(name: "foobar") { configuration { parameters { name value } } } } }
	}]]
	t.check { data = {
		team = {
			dev = {
				postgres = { configuration = { parameters = { { name = "shared_buffers", value = "128MB" } } } },
				empty = { configuration = { parameters = {} } },
				missing = { name = "without-audit", configuration = Null },
			},
			staging = { postgres = { configuration = { parameters = { { name = "work_mem", value = "8MB" } } } } },
		},
		other = { environment = { postgres = { configuration = { parameters = { { name = "work_mem", value = "16MB" } } } } } },
	} }
end)
