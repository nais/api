Helper.readK8sResources("k8s_resources/simple")

local user = User.new()
local team = Team.new("slug-1", "purpose", "#channel")
team:addMember(user)
Team.new("slug-2", "purpose", "#channel")

Test.gql("Event schema excludes raw Kubernetes details", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		{
			event: __type(name: "WorkloadEvent") { kind fields { name } }
			scaling: __type(name: "WorkloadSuccessfulRescaleEvent") { fields { name } }
			restart: __type(name: "WorkloadKillingEvent") { fields { name } }
			scalingData: __type(name: "WorkloadSuccessfulRescaleEventData") { name }
			restartData: __type(name: "WorkloadKillingEventData") { name }
			direction: __type(name: "WorkloadSuccessfulRescaleEventDirection") { kind enumValues { name } }
			audit: __type(name: "WorkloadAuditEvent") { name }
			generic: __type(name: "WorkloadGenericEvent") { name }
		}
	]]
	t.check { data = {
		event = { kind = "INTERFACE", fields = { { name = "id" }, { name = "triggeredAt" }, { name = "message" } } },
		scaling = { fields = { { name = "id" }, { name = "triggeredAt" }, { name = "message" }, { name = "newSize" }, { name = "direction" } } },
		restart = { fields = { { name = "id" }, { name = "triggeredAt" }, { name = "message" }, { name = "container" }, { name = "reason" } } },
		scalingData = Null,
		restartData = Null,
		direction = { kind = "ENUM", enumValues = { { name = "UP" }, { name = "DOWN" } } },
		audit = Null,
		generic = Null,
	} }
end)

Helper.SQLExec([[
	INSERT INTO k8s_events (uid, environment_name, involved_kind, involved_name, involved_namespace, data, reason, triggered_at)
	VALUES
		(gen_random_uuid(), 'dev', 'HorizontalPodAutoscaler', 'app-name', 'slug-1', '{"newSize":"3","direction":"up"}', 'SuccessfulRescale', '2026-01-01T12:00:00Z'),
		(gen_random_uuid(), 'dev', 'Deployment', 'app-name', 'slug-1', '{"user":"user@example.com"}', 'Patch', '2026-01-01T11:00:00Z'),
		(gen_random_uuid(), 'dev', 'Pod', 'app-name-old-pod', 'slug-1', '{"container":"app-name","reason":"liveness_probe_failed","workloadKind":"Application","workloadName":"app-name"}', 'Killing', '2026-01-01T10:00:00Z'),
		(gen_random_uuid(), 'dev', 'Naisjob', 'jobname-1', 'slug-1', '{"user":"job@example.com"}', 'Create', '2026-01-01T12:00:00Z'),
		(gen_random_uuid(), 'dev', 'CronJob', 'jobname-1', 'slug-1', '{"user":"job@example.com"}', 'Patch', '2026-01-01T11:00:00Z'),
		(gen_random_uuid(), 'prod', 'HorizontalPodAutoscaler', 'app-name', 'slug-1', '{"newSize":"3","direction":"up"}', 'SuccessfulRescale', NOW()),
		(gen_random_uuid(), 'dev', 'HorizontalPodAutoscaler', 'app-name', 'slug-2', '{"newSize":"3","direction":"up"}', 'SuccessfulRescale', NOW()),
		(gen_random_uuid(), 'dev', 'ConfigMap', 'app-name', 'slug-1', '{"user":"other@example.com"}', 'WrongKind', NOW()),
		(gen_random_uuid(), 'dev', 'Naisjob', 'app-name', 'slug-1', '{"newSize":"3","direction":"up"}', 'SuccessfulRescale', NOW()),
		(gen_random_uuid(), 'dev', 'Pod', 'app-name-unowned-pod', 'slug-1', '{"container":"app-name","reason":"liveness_probe_failed"}', 'Killing', NOW()),
		(gen_random_uuid(), 'dev', 'Application', 'another-app', 'slug-1', '{}', 'WrongWorkload', NOW())
]])

Test.gql("Application events are scoped and paginated", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		{
			team(slug: "slug-1") {
				environment(name: "dev") {
					application(name: "app-name") {
						events(first: 1) {
							nodes {
								id message __typename
								... on WorkloadSuccessfulRescaleEvent { newSize direction }
							}
							pageInfo { totalCount hasNextPage endCursor }
						}
					}
				}
			}
		}
	]]
	t.check { data = { team = { environment = { application = { events = {
		nodes = { { id = Save("eventID"), message = "Application scaled", __typename = "WorkloadSuccessfulRescaleEvent", newSize = 3, direction = "UP" } },
		pageInfo = { totalCount = 2, hasNextPage = true, endCursor = Save("eventsCursor") },
	} } } } } }
end)

Test.gql("Event node lookup", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format(
		[[{ node(id: "%s") { ... on WorkloadEvent { message } ... on WorkloadSuccessfulRescaleEvent { newSize } } }]],
		State.eventID))
	t.check { data = { node = { message = "Application scaled", newSize = 3 } } }
end)

Test.gql("Next page includes persisted pod events but not audits", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		{ team(slug: "slug-1") { environment(name: "dev") { application(name: "app-name") {
			events(first: 2, after: "%s") {
				nodes {
					id message __typename
					... on WorkloadKillingEvent { container reason }
				}
				pageInfo { totalCount hasNextPage hasPreviousPage endCursor }
			}
		} } } }
	]], State.eventsCursor))
	t.check { data = { team = { environment = { application = { events = {
		nodes = {
			{ id = Save("killingEventID"), message = "Instance restarted", __typename = "WorkloadKillingEvent", container = "app-name", reason = "liveness_probe_failed" },
		},
		pageInfo = { totalCount = 2, hasNextPage = false, hasPreviousPage = true, endCursor = Save("lastEventCursor") },
	} } } } } }
end)

Test.gql("Typed restart node lookup", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		{
			restart: node(id: "%s") { ... on WorkloadKillingEvent { container reason } }
		}
	]], State.killingEventID))
	t.check { data = {
		restart = { container = "app-name", reason = "liveness_probe_failed" },
	} }
end)

Test.gql("Empty event page preserves total count", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		{ team(slug: "slug-1") { environment(name: "dev") { application(name: "app-name") {
			events(after: "%s") { nodes { message } pageInfo { totalCount hasNextPage } }
		} } } }
	]], State.lastEventCursor))
	t.check { data = { team = { environment = { application = { events = {
		nodes = {}, pageInfo = { totalCount = 2, hasNextPage = false },
	} } } } } }
end)

Test.gql("Event time filter has inclusive from and exclusive to", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		{ team(slug: "slug-1") { environment(name: "dev") { application(name: "app-name") {
			events(filter: { from: "2026-01-01T10:00:00Z", to: "2026-01-01T12:00:00Z" }) {
				nodes { message } pageInfo { totalCount }
			}
		} } } }
	]]
	t.check { data = { team = { environment = { application = { events = {
		nodes = { { message = "Instance restarted" } }, pageInfo = { totalCount = 1 },
	} } } } } }
end)

Test.gql("Events support backwards pagination", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		{ team(slug: "slug-1") { environment(name: "dev") { application(name: "app-name") {
			events(last: 1, before: "%s") { nodes { message } pageInfo { totalCount hasPreviousPage } }
		} } } }
	]], State.lastEventCursor))
	t.check { data = { team = { environment = { application = { events = {
		nodes = { { message = "Application scaled" } }, pageInfo = { totalCount = 2, hasPreviousPage = false },
	} } } } } }
end)

Test.gql("Job events exclude job and cronjob audits", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		{ team(slug: "slug-1") { environment(name: "dev") { job(name: "jobname-1") {
			events { nodes { message } pageInfo { totalCount } }
		} } } }
	]]
	t.check { data = { team = { environment = { job = { events = {
		nodes = {}, pageInfo = { totalCount = 0 },
	} } } } } }
end)

Test.gql("Unknown event reasons are excluded", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		{ team(slug: "slug-1") { environment(name: "dev") { application(name: "another-app") {
			events { nodes { __typename message } pageInfo { totalCount } }
		} } } }
	]]
	t.check { data = { team = { environment = { application = { events = {
		nodes = {},
		pageInfo = { totalCount = 0 },
	} } } } } }
end)

Helper.SQLExec("REFRESH MATERIALIZED VIEW activity_log_subset_mat_view")

Test.gql("Kubernetes events are absent from activity logs and facets", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		{
			activityLog { pageInfo { totalCount } facets { activityTypes { activityType count } } }
			team(slug: "slug-1") {
				activityLog { pageInfo { totalCount } }
				environment(name: "dev") {
					application(name: "app-name") { activityLog { pageInfo { totalCount } } }
					job(name: "jobname-1") { activityLog { pageInfo { totalCount } } }
				}
			}
		}
	]]
	t.check { data = {
		activityLog = { pageInfo = { totalCount = 0 }, facets = { activityTypes = {} } },
		team = {
			activityLog = { pageInfo = { totalCount = 0 } },
			environment = {
				application = { activityLog = { pageInfo = { totalCount = 0 } } },
				job = { activityLog = { pageInfo = { totalCount = 0 } } },
			},
		},
	} }
end)

Helper.SQLExec([[
	WITH deployment AS (
		INSERT INTO deployments (team_slug, repository, environment_name)
		VALUES ('slug-1', 'org/repo', 'dev') RETURNING id
	)
	INSERT INTO deployment_k8s_resources (deployment_id, "group", version, kind, name, namespace)
	SELECT id, 'nais.io', 'v1alpha1', 'Application', 'app-name', 'slug-1' FROM deployment
]])
Helper.SQLExec("REFRESH MATERIALIZED VIEW activity_log_subset_mat_view")

Test.gql("Deployments remain in the activity log", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		{ team(slug: "slug-1") { environment(name: "dev") { application(name: "app-name") {
			activityLog { nodes { __typename resourceName } pageInfo { totalCount } }
			events { pageInfo { totalCount } }
		} } } }
	]]
	t.check { data = { team = { environment = { application = {
		activityLog = { nodes = { { __typename = "DeploymentActivityLogEntry", resourceName = "app-name" } }, pageInfo = { totalCount = 1 } },
		events = { pageInfo = { totalCount = 2 } },
	} } } } }
end)
