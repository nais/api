local member = User.new("postgres-crud-user", "postgres-crud-user@usersen.com")
local outsider = User.new("postgres-crud-outsider", "postgres-crud-outsider@usersen.com")
local team = Team.new("pg-crud-team", "Testing Postgres create and update", "#postgres-crud")
team:addMember(member)
Helper.readK8sResources("k8s_resources/postgres_crud")

Test.gql("Discover Postgres versions accepted in an environment", function(t)
	t.addHeader("x-user-email", member:email())
	t.query [[{ environment(name: "dev") { availableValues { id postgresMajorVersions { id values defaultValues } } } }]]
	t.check {
		data = { environment = { availableValues = { id = NotNull(), postgresMajorVersions = {
			id = Save("postgresOptionsID"), values = { "18", "17", "16" }, defaultValues = Null,
		} } } },
	}
end)

Test.gql("Discover the default Postgres version for the target environment", function(t)
	t.addHeader("x-user-email", member:email())
	t.query [[{ environment(name: "dev-gcp") { availableValues { postgresMajorVersions { values defaultValues } } } }]]
	t.check {
		data = { environment = { availableValues = { postgresMajorVersions = {
			values = { "18", "17" }, defaultValues = { "17" },
		} } } },
	}
end)

Test.gql("Discover Postgres extensions accepted in an environment", function(t)
	t.addHeader("x-user-email", member:email())
	t.query [[{ environment(name: "dev") { availableValues { postgresExtensions { values defaultValues } } } }]]
	t.check {
		data = { environment = { availableValues = { postgresExtensions = {
			values = { "tablefunc", "citext", "pg_trgm", "pgaudit", "postgis", "timescaledb" },
			defaultValues = Null,
		} } } },
	}
end)

Test.gql("String options can be fetched by node ID", function(t)
	t.addHeader("x-user-email", member:email())
	t.query(string.format([[{ node(id: "%s") { ... on AvailableValueOptions { id values defaultValues } } }]],
		State.postgresOptionsID))
	t.check {
		data = { node = {
			id = State.postgresOptionsID, values = { "18", "17", "16" }, defaultValues = Null,
		} },
	}
end)

Test.gql("Missing version catalog is an availability error, not an empty list", function(t)
	t.addHeader("x-user-email", member:email())
	t.query [[{ environment(name: "staging") { availableValues { postgresMajorVersions { values defaultValues } } } }]]
	t.check {
		errors = { { locations = NotNull(), path = { "environment", "availableValues", "postgresMajorVersions" }, message = Contains("unavailable") } },
		data = Null,
	}
end)

local function create(name, extra)
	return string.format(
		[[mutation { createPostgres(input: {
			name: "%s", environmentName: "dev", teamSlug: "pg-crud-team", %s
		}) { postgres { name majorVersion extensions highAvailability resources { cpu memory diskSize } } } }]],
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
					extensions = {},
					highAvailability = false,
					resources = { cpu = Null, memory = Null, diskSize = Null },
				},
			},
		},
	}
end)

Test.gql("Team members can create Postgres with extensions", function(t)
	t.addHeader("x-user-email", member:email())
	t.query(create("extended", 'majorVersion: "18", extensions: ["pg_trgm", "citext"]'))
	t.check {
		data = { createPostgres = { postgres = {
			name = "extended",
			majorVersion = "18",
			extensions = { "pg_trgm", "citext" },
			highAvailability = false,
			resources = { cpu = Null, memory = Null, diskSize = Null },
		} } },
	}
end)

Test.gql("Team members can create Postgres with explicit resources", function(t)
	t.addHeader("x-user-email", member:email())
	t.query(create("sized", 'majorVersion: "18", highAvailability: true, cpu: "250m", memory: "1Gi", diskSize: "20Gi"'))
	t.check {
		data = {
			createPostgres = {
				postgres = {
					name = "sized",
					majorVersion = "18",
					extensions = {},
					highAvailability = true,
					resources = { cpu = "250m", memory = "1Gi", diskSize = "20Gi" },
				},
			},
		},
	}
end)

Test.gql("Creating an existing Postgres fails", function(t)
	t.addHeader("x-user-email", member:email())
	t.query(create("existing", 'majorVersion: "18"'))
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

Test.gql("Empty major version is rejected", function(t)
	t.addHeader("x-user-email", member:email())
	t.query(create("emptyversion", 'majorVersion: ""'))
	t.check {
		errors = { { extensions = { field = "majorVersion" }, path = { "createPostgres" }, message = Contains("Major version must not be empty") } },
		data = Null,
	}
end)

Test.gql("Creating Postgres requires an available version catalog", function(t)
	t.addHeader("x-user-email", member:email())
	for _, environment in ipairs({ "staging", "unknown" }) do
		t.query(string.format(
			[[mutation { createPostgres(input: {
				name: "unavailable", environmentName: "%s", teamSlug: "pg-crud-team", majorVersion: "18"
			}) { postgres { name } } }]],
			environment
		))
		t.check {
			errors = { { locations = NotNull(), path = { "createPostgres" }, message = Contains("unavailable") } },
			data = Null,
		}
	end
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

Test.gql("Names that Kubernetes would reject are rejected as field errors", function(t)
	t.addHeader("x-user-email", member:email())
	for _, name in ipairs({ "Upper", "has/slash", "-leading" }) do
		t.query(create(name, 'majorVersion: "18"'))
		t.check {
			errors = { { extensions = { field = "name" }, path = { "createPostgres" }, message = Contains("lowercase letters") } },
			data = Null,
		}
	end
	t.query [[mutation { updatePostgres(input: {
		name: "Upper", environmentName: "dev", teamSlug: "pg-crud-team", highAvailability: true
	}) { postgres { name } } }]]
	t.check {
		errors = { { extensions = { field = "name" }, path = { "updatePostgres" }, message = Contains("lowercase letters") } },
		data = Null,
	}
end)

Test.gql("Zero and negative resource quantities are rejected", function(t)
	t.addHeader("x-user-email", member:email())
	for _, field in ipairs({ "cpu", "memory", "diskSize" }) do
		t.query(create("badquantity", string.format('majorVersion: "18", %s: "-1"', field)))
		t.check {
			errors = { { extensions = { field = field }, path = { "createPostgres" }, message = Contains("must be greater than zero") } },
			data = Null,
		}
	end
end)

Test.gql("Creating and updating Postgres is recorded in the activity log", function(t)
	t.addHeader("x-user-email", member:email())
	t.query [[{ team(slug: "pg-crud-team") {
		activityLog(first: 50, filter: { activityTypes: [POSTGRES_CREATED, POSTGRES_UPDATED] }) {
			nodes { __typename message actor resourceType resourceName environmentName teamSlug
				... on PostgresUpdatedActivityLogEntry { data { updatedFields { field oldValue newValue } } }
			}
		}
	} }]]
	t.check {
		data = { team = { activityLog = { nodes = {
			{ __typename = "PostgresUpdatedActivityLogEntry", message = "Updated Postgres", actor = member:email(), resourceType = "POSTGRES", resourceName = "existing", environmentName = "dev", teamSlug = "pg-crud-team",
				data = { updatedFields = {
					{ field = "highAvailability", oldValue = "false", newValue = "true" },
					{ field = "diskSize",         oldValue = "10Gi",  newValue = "30Gi" },
				} },
			},
			{ __typename = "PostgresCreatedActivityLogEntry", message = "Created Postgres", actor = member:email(), resourceType = "POSTGRES", resourceName = Ignore(), environmentName = "dev", teamSlug = "pg-crud-team" },
			{ __typename = "PostgresCreatedActivityLogEntry", message = "Created Postgres", actor = member:email(), resourceType = "POSTGRES", resourceName = Ignore(), environmentName = "dev", teamSlug = "pg-crud-team" },
		} } } },
	}
end)

Test.gql("Postgres activity log excludes other Postgres instances", function(t)
	t.addHeader("x-user-email", member:email())
	t.query [[{ team(slug: "pg-crud-team") { environment(name: "dev") { postgres(name: "existing") {
		activityLog(first: 5) {
			nodes { __typename resourceName teamSlug environmentName }
			pageInfo { hasNextPage }
		}
	} } } }]]
	t.check {
		data = { team = { environment = { postgres = { activityLog = {
			nodes = {
				{ __typename = "PostgresUpdatedActivityLogEntry", resourceName = "existing", teamSlug = "pg-crud-team", environmentName = "dev" },
			},
			pageInfo = { hasNextPage = false },
		} } } } },
	}
end)

Test.gql("Every major version offered by the environment can be selected", function(t)
	t.addHeader("x-user-email", member:email())
	for _, version in ipairs({ "17", "16" }) do
		t.query(create("version-" .. version, string.format('majorVersion: "%s"', version)))
		t.check {
			data = { createPostgres = { postgres = {
				name = "version-" .. version,
				majorVersion = version,
				extensions = {},
				highAvailability = false,
				resources = { cpu = Null, memory = Null, diskSize = Null },
			} } },
		}
	end
end)

Test.gql("Team members can replace and clear Postgres extensions", function(t)
	t.addHeader("x-user-email", member:email())
	t.query [[mutation { updatePostgres(input: {
		name: "existing", environmentName: "dev", teamSlug: "pg-crud-team",
		extensions: ["citext", "pg_trgm"]
	}) { postgres { extensions } } }]]
	t.check {
		data = { updatePostgres = { postgres = { extensions = { "citext", "pg_trgm" } } } },
	}

	t.query [[mutation { updatePostgres(input: {
		name: "existing", environmentName: "dev", teamSlug: "pg-crud-team",
		extensions: []
	}) { postgres { extensions } } }]]
	t.check {
		data = { updatePostgres = { postgres = { extensions = {} } } },
	}
end)
