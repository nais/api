local user = User.new("bucket-user", "bucket-user@usersen.com")
local team = Team.new("bucket-team", "Testing storage buckets", "#buckets")
team:addMember(user)
Helper.readK8sResources("k8s_resources/buckets")

Test.k8s("The ignored Postgres bucket exists in Kubernetes", function(t)
	t.check("storage.cnrm.cloud.google.com/v1beta1", "storagebuckets", "dev", "bucket-team", "postgres-backup", {
		apiVersion = "storage.cnrm.cloud.google.com/v1beta1",
		kind = "StorageBucket",
		metadata = {
			name = "postgres-backup",
			namespace = "bucket-team",
			annotations = { ["cnrm.cloud.google.com/project-id"] = "bucket-project" },
			labels = {
				["postgres.nais.io/name"] = "orders",
				purpose = "backup",
				team = "bucket-team",
			},
		},
		spec = { publicAccessPrevention = "enforced", uniformBucketLevelAccess = true },
	})
end)

Test.gql("List buckets and inventory without Postgres backups", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{
		team(slug: "bucket-team") {
			buckets {
				pageInfo { totalCount }
				nodes {
					id name cascadingDelete publicAccessPrevention uniformBucketLevelAccess
					team { slug }
					teamEnvironment { environment { name } }
					labels { key value }
				}
				facets {
					environments { value count }
					labels { key value count }
				}
			}
			inventoryCounts { buckets { total } }
		}
	}]]
	t.check {
		data = {
			team = {
				buckets = {
					pageInfo = { totalCount = 3 },
					nodes = {
						{
							id = NotNull(), name = "assets", cascadingDelete = true,
							publicAccessPrevention = "enforced", uniformBucketLevelAccess = true,
							team = { slug = "bucket-team" },
							teamEnvironment = { environment = { name = "dev" } },
							labels = { { key = "purpose", value = "assets" } },
						},
						{
							id = NotNull(), name = "staging-assets", cascadingDelete = true,
							publicAccessPrevention = "enforced", uniformBucketLevelAccess = true,
							team = { slug = "bucket-team" },
							teamEnvironment = { environment = { name = "staging" } },
							labels = { { key = "purpose", value = "assets" } },
						},
						{
							id = NotNull(), name = "uploads", cascadingDelete = false,
							publicAccessPrevention = "inherited", uniformBucketLevelAccess = false,
							team = { slug = "bucket-team" },
							teamEnvironment = { environment = { name = "dev" } },
							labels = { { key = "purpose", value = "uploads" } },
						},
					},
					facets = {
						environments = {
							{ value = "dev",     count = 2 },
							{ value = "staging", count = 1 },
						},
						labels = {
							{ key = "purpose", value = "assets",  count = 2 },
							{ key = "purpose", value = "uploads", count = 1 },
						},
					},
				},
				inventoryCounts = { buckets = { total = 3 } },
			},
		},
	}
end)

Test.gql("Filter buckets by name, environment and label", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{
		team(slug: "bucket-team") {
			buckets(filter: {
				name: "ASSETS", environments: ["dev"],
				labels: [{ key: "purpose", value: "assets" }]
			}) {
				pageInfo { totalCount }
				nodes { name }
			}
		}
	}]]
	t.check { data = { team = { buckets = {
		pageInfo = { totalCount = 1 },
		nodes = { { name = "assets" } },
	} } } }
end)

Test.gql("Order and paginate buckets", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{
		team(slug: "bucket-team") {
			buckets(first: 2, orderBy: { field: NAME, direction: DESC }) {
				pageInfo { totalCount hasNextPage }
				nodes { name }
			}
		}
	}]]
	t.check { data = { team = { buckets = {
		pageInfo = { totalCount = 3, hasNextPage = true },
		nodes = { { name = "uploads" }, { name = "staging-assets" } },
	} } } }
end)

Test.gql("Get a bucket in its team environment", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{
		team(slug: "bucket-team") {
			environment(name: "dev") {
				bucket(name: "assets") { name publicAccessPrevention uniformBucketLevelAccess }
			}
		}
	}]]
	t.check { data = { team = { environment = { bucket = {
		name = "assets", publicAccessPrevention = "enforced", uniformBucketLevelAccess = true,
	} } } } }
end)

Test.gql("An ignored Postgres bucket cannot be looked up", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{
		team(slug: "bucket-team") {
			environment(name: "dev") { bucket(name: "postgres-backup") { name } }
		}
	}]]
	t.check {
		data = Null,
		errors = {
			{
				locations = NotNull(),
				message = Contains("Resource not found: dev/bucket-team/postgres-backup"),
				path = { "team", "environment", "bucket" },
			},
		},
	}
end)

Test.gql("Workload bucket references exclude Postgres backups", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{
		team(slug: "bucket-team") {
			environment(name: "dev") {
				application(name: "bucket-app") { buckets { nodes { name } } }
				job(name: "bucket-job") { buckets { nodes { name } } }
			}
		}
	}]]
	t.check { data = { team = { environment = {
		application = { buckets = { nodes = { { name = "assets" } } } },
		job = { buckets = { nodes = { { name = "uploads" } } } },
	} } } }
end)

Test.gql("Search finds regular buckets but not Postgres backups", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{
		visible: search(filter: { query: "assets", types: [BUCKET], teams: ["bucket-team"] }) {
			nodes { ... on Bucket { name } }
		}
		ignored: search(filter: { query: "postgres-backup", types: [BUCKET], teams: ["bucket-team"] }) {
			nodes { __typename }
		}
	}]]
	t.check { data = {
		visible = { nodes = { { name = "assets" }, { name = "staging-assets" } } },
		ignored = { nodes = {} },
	} }
end)
