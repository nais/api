local member = User.new("backup-member", "backup-member@usersen.com")
local outsider = User.new("backup-outsider", "backup-outsider@usersen.com")
local team = Team.new("pg-crud-team", "Postgres backups", "#backups")
team:addMember(member)
Helper.readK8sResources("k8s_resources/postgres_crud")
Helper.readK8sResources("k8s_resources/postgres_backups")

Test.gql("A retained backup is visible without its source database", function(t)
	t.addHeader("x-user-email", member:email())
	t.query [[{ team(slug: "pg-crud-team") { environment(name: "dev") {
		postgresBackup(name: "retained-backup") { name sourcePostgres phase majorVersion diskSize extensions sizeBytes completedAt expiresAt }
		postgresBackups(first: 1, filter: { sourcePostgres: "deleted-source" }) { nodes { name } pageInfo { totalCount } }
	} } }]]
	t.check { data = { team = { environment = {
		postgresBackup = { name = "retained-backup", sourcePostgres = "deleted-source", phase = "COMPLETED",
			majorVersion = "17", diskSize = "20Gi", extensions = { "vector" },
			sizeBytes = 5368709120, completedAt = "2026-11-25T10:01:00Z", expiresAt = "2026-12-25T10:01:00Z" },
		postgresBackups = { nodes = { { name = "retained-backup" } }, pageInfo = { totalCount = 1 } },
	} } } }
end)

Test.gql("Only team members can read standalone backups", function(t)
	t.addHeader("x-user-email", outsider:email())
	t.query [[{ team(slug: "pg-crud-team") { environment(name: "dev") { postgresBackup(name: "retained-backup") { name } } } }]]
	t.check { data = Null, errors = { { locations = NotNull(), path = { "team", "environment", "postgresBackup" }, message = Contains("You are authenticated") } } }
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
