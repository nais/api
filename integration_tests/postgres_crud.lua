local member = User.new("postgres-crud-user", "postgres-crud-user@usersen.com")
local outsider = User.new("postgres-crud-outsider", "postgres-crud-outsider@usersen.com")
local team = Team.new("pg-crud-team", "Testing Postgres create and update", "#postgres-crud")
team:addMember(member)
Helper.readK8sResources("k8s_resources/postgres_crud")

local function create(name, extra)
	return string.format(
		[[mutation { createPostgres(input: {
			name: "%s", environmentName: "dev", teamSlug: "pg-crud-team", %s
		}) { postgres { name majorVersion highAvailability resources { cpu memory diskSize } } } }]],
		name,
		extra
	)
end

Test.gql("Non-members cannot create Postgres", function(t)
	t.addHeader("x-user-email", outsider:email())
	t.query(create("denied", 'majorVersion: "18"'))
	t.check {
		errors = { { locations = NotNull(), path = { "createPostgres" }, message = Contains("You are authenticated") } },
		data = Null,
	}
end)

Test.gql("Team members can create Postgres with defaults left to the platform", function(t)
	t.addHeader("x-user-email", member:email())
	t.query(create("fresh", 'majorVersion: "18"'))
	t.check {
		data = {
			createPostgres = {
				postgres = {
					name = "fresh",
					majorVersion = "18",
					highAvailability = false,
					resources = { cpu = Null, memory = Null, diskSize = Null },
				},
			},
		},
	}
end)

Test.gql("Team members can create Postgres with explicit resources", function(t)
	t.addHeader("x-user-email", member:email())
	t.query(create("sized", 'majorVersion: "16", highAvailability: true, cpu: "250m", memory: "1Gi", diskSize: "20Gi"'))
	t.check {
		data = {
			createPostgres = {
				postgres = {
					name = "sized",
					majorVersion = "16",
					highAvailability = true,
					resources = { cpu = "250m", memory = "1Gi", diskSize = "20Gi" },
				},
			},
		},
	}
end)

Test.gql("Creating an existing Postgres fails", function(t)
	t.addHeader("x-user-email", member:email())
	t.query(create("existing", 'majorVersion: "17"'))
	t.check {
		errors = { { locations = NotNull(), path = { "createPostgres" }, message = NotNull() } },
		data = Null,
	}
end)

Test.gql("Unsupported major version is rejected", function(t)
	t.addHeader("x-user-email", member:email())
	t.query(create("badversion", 'majorVersion: "9"'))
	t.check {
		errors = { { extensions = { field = "majorVersion" }, path = { "createPostgres" }, message = Contains("Major version must be one of") } },
		data = Null,
	}
end)

Test.gql("Invalid resource quantity is rejected", function(t)
	t.addHeader("x-user-email", member:email())
	t.query(create("badquantity", 'majorVersion: "18", cpu: "lots"'))
	t.check {
		errors = { { extensions = { field = "cpu" }, path = { "createPostgres" }, message = Contains("is not a valid quantity") } },
		data = Null,
	}
end)

Test.gql("Team members can update only the fields they provide", function(t)
	t.addHeader("x-user-email", member:email())
	t.query [[mutation { updatePostgres(input: {
		name: "existing", environmentName: "dev", teamSlug: "pg-crud-team", highAvailability: true, diskSize: "30Gi"
	}) { postgres { majorVersion highAvailability resources { cpu memory diskSize } } } }]]
	t.check {
		data = {
			updatePostgres = {
				postgres = {
					majorVersion = "17",
					highAvailability = true,
					resources = { cpu = "100m", memory = "512Mi", diskSize = "30Gi" },
				},
			},
		},
	}
end)

Test.gql("Non-members cannot update Postgres", function(t)
	t.addHeader("x-user-email", outsider:email())
	t.query [[mutation { updatePostgres(input: {
		name: "existing", environmentName: "dev", teamSlug: "pg-crud-team", highAvailability: true
	}) { postgres { name } } }]]
	t.check {
		errors = { { locations = NotNull(), path = { "updatePostgres" }, message = Contains("You are authenticated") } },
		data = Null,
	}
end)

Test.gql("Updating a missing Postgres fails", function(t)
	t.addHeader("x-user-email", member:email())
	t.query [[mutation { updatePostgres(input: {
		name: "missing", environmentName: "dev", teamSlug: "pg-crud-team", highAvailability: true
	}) { postgres { name } } }]]
	t.check {
		errors = { { locations = NotNull(), path = { "updatePostgres" }, message = Contains("not found") } },
		data = Null,
	}
end)
