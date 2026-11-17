local member = User.new("pitr-member", "pitr-member@usersen.com")
local outsider = User.new("pitr-outsider", "pitr-outsider@usersen.com")
local team = Team.new("pg-crud-team", "Postgres PITR", "#pitr")
team:addMember(member)
local otherTeam = Team.new("pg-pitr-other", "PITR source isolation", "#pitr-other")
otherTeam:addMember(member)
Helper.readK8sResources("k8s_resources/postgres_crud")
Helper.readK8sResources("k8s_resources/postgres_pitr")

local function restore(name, source, target, extra)
	return string.format([[mutation { createPostgres(input: {
		teamSlug: "pg-crud-team", environmentName: "dev", name: "%s", majorVersion: "18",
		recovery: { sourcePostgres: "%s", targetTime: "%s" }, %s
	}) { postgres { name extensions highAvailability resources { cpu memory diskSize }
		recovery { sourcePostgres targetTime completed failureMessage } } } }]], name, source, target, extra or "")
end

Test.gql("PITR source exposes the preserved physical archive lower bound", function(t)
	t.addHeader("x-user-email", member:email())
	t.query [[{ team(slug: "pg-crud-team") { environment(name: "dev") { postgres(name: "pitr-source") { earliestRestoreTime } } } }]]
	t.check { data = { team = { environment = { postgres = { earliestRestoreTime = "2026-09-29T00:00:00Z" } } } } }
end)

Test.gql("PITR uses normal create authorization", function(t)
	t.addHeader("x-user-email", outsider:email())
	t.query(restore("denied-restore", "pitr-source", "2026-09-30T12:00:00Z"))
	t.check { data = Null, errors = { { locations = NotNull(), path = { "createPostgres" }, message = Contains("You are authenticated") } } }
end)

Test.gql("PITR creates an independent Postgres and inherits source configuration", function(t)
	t.addHeader("x-user-email", member:email())
	t.query(restore("pitr-restored", "pitr-source", "2026-09-30T14:00:00+02:00"))
	t.check { data = { createPostgres = { postgres = {
		name = "pitr-restored", extensions = { "pg_trgm" }, highAvailability = true,
		resources = { cpu = "250m", memory = "1Gi", diskSize = "20Gi" },
		recovery = { sourcePostgres = "pitr-source", targetTime = "2026-09-30T12:00:00Z", completed = false, failureMessage = Null },
	} } } }
end)

Test.k8s("PITR pins source UID and physical archive identity", function(t)
	t.check("nais.io/v1", "postgres", "dev", team:slug(), "pitr-restored", {
		apiVersion = "nais.io/v1",
		kind = "Postgres",
		metadata = Ignore(),
		spec = {
			majorVersion = "18",
			highAvailability = true,
			extensions = { { name = "pg_trgm" } },
			resources = { cpu = "250m", memory = "1Gi", diskSize = "20Gi" },
			recovery = { sourcePostgres = "pitr-source", sourceUID = "pitr-source-uid", bucketName = "pitr-source-archive", serverName = "pg-pitr-source-main", targetTime = "2026-09-30T12:00:00Z" },
		},
	})
end)

for _, case in ipairs({
	{ name = "before archive",           source = "pitr-source", time = "2026-09-28T12:00:00Z", message = "earliest recoverable" },
	{ name = "missing source",           source = "unknown",     time = "2026-09-30T12:00:00Z", message = "not found in this team and environment" },
	{ name = "missing physical source",  source = "branchless",  time = "2026-09-30T12:00:00Z", message = "no recoverable cluster" },
	{ name = "smaller disk",             source = "pitr-source", time = "2026-09-30T12:00:00Z", extra = 'diskSize: "10Gi"',                        message = "must not be smaller" },
	{ name = "missing source extension", source = "pitr-source", time = "2026-09-30T12:00:00Z", extra = "extensions: []",                          message = "must include source extension" },
}) do
	Test.gql("PITR rejects " .. case.name, function(t)
		t.addHeader("x-user-email", member:email())
		t.query(restore("invalid-restore", case.source, case.time, case.extra))
		t.check { data = Null, errors = { { locations = NotNull(), path = { "createPostgres" }, message = Contains(case.message) } } }
	end)
end

Test.gql("PITR cannot change PostgreSQL major version", function(t)
	t.addHeader("x-user-email", member:email())
	t.query [[mutation { createPostgres(input: {
		teamSlug: "pg-crud-team", environmentName: "dev", name: "wrong-major", majorVersion: "17",
		recovery: { sourcePostgres: "pitr-source", targetTime: "2026-09-30T12:00:00Z" }
	}) { postgres { name } } }]]
	t.check { data = Null, errors = { { locations = NotNull(), path = { "createPostgres" }, message = Contains("source PostgreSQL major version 18") } } }
end)

Test.gql("PITR source lookup cannot cross environments", function(t)
	t.addHeader("x-user-email", member:email())
	t.query [[mutation { createPostgres(input: {
		teamSlug: "pg-crud-team", environmentName: "dev-gcp", name: "cross-env", majorVersion: "18",
		recovery: { sourcePostgres: "pitr-source", targetTime: "2026-09-30T12:00:00Z" }
	}) { postgres { name } } }]]
	t.check { data = Null, errors = { { locations = NotNull(), path = { "createPostgres" }, message = Contains("not found in this team and environment") } } }
end)

Test.gql("PITR creation is audited with source and UTC target", function(t)
	t.addHeader("x-user-email", member:email())
	t.query [[{ team(slug: "pg-crud-team") { activityLog(filter: { activityTypes: [POSTGRES_CREATED] }) { nodes { message } } } }]]
	t.check { data = { team = { activityLog = { nodes = { { message = "Requested Postgres recovery from pitr-source to 2026-09-30T12:00:00Z" } } } } } }
end)

Test.gql("PITR source lookup cannot cross teams even for a member of both", function(t)
	t.addHeader("x-user-email", member:email())
	t.query [[mutation { createPostgres(input: {
		teamSlug: "pg-pitr-other", environmentName: "dev", name: "cross-team", majorVersion: "18",
		recovery: { sourcePostgres: "pitr-source", targetTime: "2026-09-30T12:00:00Z" }
	}) { postgres { name } } }]]
	t.check { data = Null, errors = { { locations = NotNull(), path = { "createPostgres" }, message = Contains("not found in this team and environment") } } }
end)
