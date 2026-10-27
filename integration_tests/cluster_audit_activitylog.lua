local user = User.new()
local team = Team.new("audit-team", "Audit team", "#audit-team")
team:addMember(user)

Helper.SQLExec([[
	INSERT INTO activity_log_entries (
		actor, action, resource_type, resource_name, team_slug, environment, created_at, data, kubernetes_audit_uid
	)
	VALUES (
		'user@example.com', 'CLUSTER_AUDIT', 'CLUSTER_AUDIT', 'app', 'audit-team', 'dev',
		'2026-01-01T12:00:00Z', CONVERT_TO('{"action":"Patch","resourceKind":"Deployment"}', 'UTF8'),
		'11111111-1111-1111-1111-111111111111'
	)
]])

Test.gql("Cluster audits are available in the activity log", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		{
			activityLog(filter: { activityTypes: [CLUSTER_AUDIT] }) {
				nodes {
					id __typename actor createdAt message resourceName teamSlug environmentName
					... on ClusterAuditActivityLogEntry { data { action resourceKind } }
				}
				pageInfo { totalCount }
				facets { activityTypes { activityType count } }
			}
			team(slug: "audit-team") {
				activityLog(filter: { activityTypes: [CLUSTER_AUDIT] }) { pageInfo { totalCount } }
			}
		}
	]]
	t.check { data = {
		activityLog = {
			nodes = { {
				id = Save("auditID"), __typename = "ClusterAuditActivityLogEntry", actor = "user@example.com",
				createdAt = "2026-01-01T12:00:00Z", message = "Patched Deployment app", resourceName = "app",
				teamSlug = "audit-team", environmentName = "dev", data = { action = "Patch", resourceKind = "Deployment" },
			} },
			pageInfo = { totalCount = 1 },
			facets = { activityTypes = { { activityType = "CLUSTER_AUDIT", count = 1 } } },
		},
		team = { activityLog = { pageInfo = { totalCount = 1 } } },
	} }
end)

Helper.SQLExec([[
	INSERT INTO activity_log_entries (
		actor, action, resource_type, resource_name, team_slug, environment, data
	)
	VALUES (
		'user@example.com', 'CLUSTER_AUDIT', 'CLUSTER_AUDIT', 'long-namespace-app',
		'namespace-that-is-longer-than-thirty-characters', 'dev',
		CONVERT_TO('{"action":"Patch","resourceKind":"Deployment"}', 'UTF8')
	)
]])

Test.gql("Audit namespaces longer than team slugs are preserved", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		{ activityLog(first: 1, filter: { activityTypes: [CLUSTER_AUDIT] }) { nodes { resourceName teamSlug } } }
	]]
	t.check { data = { activityLog = { nodes = { {
		resourceName = "long-namespace-app", teamSlug = "namespace-that-is-longer-than-thirty-characters",
	} } } } }
end)

Test.gql("Cluster audit node lookup", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		{ node(id: "%s") { ... on ClusterAuditActivityLogEntry { message data { action resourceKind } } } }
	]], State.auditID))
	t.check { data = { node = { message = "Patched Deployment app", data = { action = "Patch", resourceKind = "Deployment" } } } }
end)
