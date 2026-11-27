local member = User.new("backup-member", "backup-member@usersen.com")
local outsider = User.new("backup-outsider", "backup-outsider@usersen.com")
local team = Team.new("pg-crud-team", "Postgres backups", "#backups")
team:addMember(member)
Helper.readK8sResources("k8s_resources/postgres_crud")
Helper.readK8sResources("k8s_resources/postgres_backups")

Test.gql("A retained backup is visible without its source database", function(t)
	t.addHeader("x-user-email", member:email())
	t.query [[{ team(slug: "pg-crud-team") {
		environment(name: "dev") {
			postgresBackup(name: "retained-backup") {
				name sourcePostgres phase majorVersion diskSize extensions sizeBytes completedAt expiresAt
				teamEnvironment { environment { name } }
			}
		}
		postgresBackups(first: 1, filter: { environments: ["dev"], sourcePostgres: "deleted-source" }) {
			nodes { name teamEnvironment { environment { name } } }
			pageInfo { totalCount }
		}
		unsup: postgresBackups(filter: { environments: ["dev-fss"] }) {
			pageInfo { totalCount }
		}
	} }]]
	t.check { data = { team = {
		environment = { postgresBackup = {
			name = "retained-backup", sourcePostgres = "deleted-source", phase = "COMPLETED",
			majorVersion = "17", diskSize = "20Gi", extensions = { "vector" },
			sizeBytes = 5368709120, completedAt = "2026-11-25T10:01:00Z", expiresAt = "2026-12-25T10:01:00Z",
			teamEnvironment = { environment = { name = "dev" } },
		} },
		postgresBackups = {
			nodes = { { name = "retained-backup", teamEnvironment = { environment = { name = "dev" } } } },
			pageInfo = { totalCount = 1 },
		},
		unsup = { pageInfo = { totalCount = 0 } },
	} } }
end)

Test.gql("Only team members can read standalone backups", function(t)
	t.addHeader("x-user-email", outsider:email())
	t.query [[{ team(slug: "pg-crud-team") { environment(name: "dev") { postgresBackup(name: "retained-backup") { name } } } }]]
	t.check { data = Null, errors = { { locations = NotNull(), path = { "team", "environment", "postgresBackup" }, message = Contains("You are authenticated") } } }
end)

Test.gql("Non-members cannot create or delete standalone backups", function(t)
	t.addHeader("x-user-email", outsider:email())
	t.query [[mutation { createPostgresBackup(input: {
		teamSlug: "pg-crud-team", environmentName: "dev", name: "denied-backup", sourcePostgres: "existing"
	}) { backup { name } } }]]
	t.check { data = Null, errors = { { locations = NotNull(), path = { "createPostgresBackup" }, message = Contains("You are authenticated") } } }

	t.query [[mutation { deletePostgresBackup(input: {
		teamSlug: "pg-crud-team", environmentName: "dev", name: "retained-backup"
	}) { deletionRequested } }]]
	t.check { data = Null, errors = { { locations = NotNull(), path = { "deletePostgresBackup" }, message = Contains("You are authenticated") } } }
end)

Test.gql("A team member can request a standalone backup", function(t)
	t.addHeader("x-user-email", member:email())
	t.query [[mutation { createPostgresBackup(input: {
		teamSlug: "pg-crud-team", environmentName: "dev", name: "fresh-backup", sourcePostgres: "existing", retentionDays: 7
	}) { backup { name sourcePostgres retentionDays phase } } }]]
	t.check { data = { createPostgresBackup = { backup = {
		name = "fresh-backup", sourcePostgres = "existing", retentionDays = 7, phase = "PREPARING",
	} } } }
end)

Test.k8s("Creating a standalone backup writes a separate resource", function(t)
	t.check("nais.io/v1", "postgresbackups", "dev", team:slug(), "fresh-backup", {
		apiVersion = "nais.io/v1",
		kind = "PostgresBackup",
		metadata = Ignore(),
		spec = { postgres = "existing", retentionDays = 7 },
	})
end)

Test.gql("A team member can restore a new Postgres from a completed standalone backup after the source is gone",
	function(t)
		t.addHeader("x-user-email", member:email())
		t.query [[mutation { createPostgres(input: {
		teamSlug: "pg-crud-team", environmentName: "dev", name: "restored-from-backup", majorVersion: "17",
		backupName: "retained-backup"
	}) { postgres {
		name majorVersion extensions resources { diskSize }
		recovery { sourcePostgres backupName targetTime completed failureMessage }
	} } }]]
		t.check { data = { createPostgres = { postgres = {
			name = "restored-from-backup", majorVersion = "17", extensions = { "vector" },
			resources = { diskSize = "20Gi" },
			recovery = {
				sourcePostgres = "deleted-source", backupName = "retained-backup",
				targetTime = Null, completed = false, failureMessage = Null,
			},
		} } } }
	end)

Test.k8s("Restoring from a standalone backup creates a clean Postgres and companion PostgresRecovery", function(t)
	t.check("nais.io/v1", "postgres", "dev", team:slug(), "restored-from-backup", {
		apiVersion = "nais.io/v1",
		kind = "Postgres",
		metadata = Ignore(),
		spec = {
			majorVersion = "17",
			extensions = { { name = "vector" } },
			resources = { diskSize = "20Gi" },
		},
	})
	t.check("nais.io/v1", "postgresrecoveries", "dev", team:slug(), "restored-from-backup", {
		apiVersion = "nais.io/v1",
		kind = "PostgresRecovery",
		metadata = Ignore(),
		spec = {
			postgres = "restored-from-backup",
			backup = {
				sourcePostgres = "deleted-source",
				backupName = "retained-backup",
				backupUID = "retained-backup-uid",
				bucketName = "private-bucket",
				prefix = "retained-backup",
				serverName = "pg-deleted-source-main",
				backupID = "20261125T100000",
			},
		},
	})
end)

Test.gql("Restoring from a standalone backup is rejected when not completed or disk is smaller", function(t)
	t.addHeader("x-user-email", member:email())
	t.query [[mutation { createPostgres(input: {
		teamSlug: "pg-crud-team", environmentName: "dev", name: "restore-incomplete", majorVersion: "17",
		backupName: "fresh-backup"
	}) { postgres { name } } }]]
	t.check { data = Null, errors = { { locations = NotNull(), path = { "createPostgres" }, message = Contains("not completed") } } }

	t.query [[mutation { createPostgres(input: {
		teamSlug: "pg-crud-team", environmentName: "dev", name: "restore-small-disk", majorVersion: "17",
		backupName: "retained-backup", diskSize: "10Gi"
	}) { postgres { name } } }]]
	t.check { data = Null, errors = { { locations = NotNull(), path = { "createPostgres" }, message = Contains("must not be smaller than the backup") } } }
end)

Test.gql("Backup deletion does not require the source database", function(t)
	t.addHeader("x-user-email", member:email())
	t.query [[mutation { deletePostgresBackup(input: {
		teamSlug: "pg-crud-team", environmentName: "dev", name: "retained-backup"
	}) { deletionRequested } }]]
	t.check { data = { deletePostgresBackup = { deletionRequested = true } } }
end)

Test.gql("Standalone backup requests are audited", function(t)
	t.addHeader("x-user-email", member:email())
	t.query [[{ team(slug: "pg-crud-team") { activityLog(filter: {
		activityTypes: [POSTGRES_BACKUP_CREATED, POSTGRES_BACKUP_DELETION_REQUESTED]
	}) { pageInfo { totalCount } } } }]]
	t.check { data = { team = { activityLog = { pageInfo = { totalCount = 2 } } } } }
end)
