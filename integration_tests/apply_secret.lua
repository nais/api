local user = User.new()
local outsider = User.new()
local team = Team.new("apply-secret-team", "Secret apply testing", "#apply-secret-team")
team:addMember(user)

Test.rest("create Secret via apply", function(t)
	t.addHeader("x-user-email", user:email())
	t.send("POST", "/api/v1/teams/apply-secret-team/environments/dev/apply", [[
		{"resources":[{
			"apiVersion":"v1",
			"kind":"Secret",
			"metadata":{"name":"apply-secret","annotations":{"snapshot":"OLD_SNAPSHOT"}},
			"data":{"PASSWORD":"T0xEX01BUktFUg=="},
			"stringData":{"TOKEN":"OLD_PLAINTEXT_MARKER"}
		}]}
	]])
	t.check(200, {
		results = {
			{ resource = "Secret/apply-secret", environmentName = "dev", status = "created" },
		},
	})
end)

local update = [[
	{"resources":[{
		"apiVersion":"v1",
		"kind":"Secret",
		"metadata":{"name":"apply-secret","annotations":{"snapshot":"NEW_SNAPSHOT"}},
		"data":{"PASSWORD":"TkVXX01BUktFUg=="},
		"stringData":{"TOKEN":"NEW_PLAINTEXT_MARKER"}
	}]}
]]

Test.rest("Secret apply response contains field paths only", function(t)
	t.addHeader("x-user-email", user:email())
	t.send("POST", "/api/v1/teams/apply-secret-team/environments/dev/apply", update)
	t.check(200, {
		results = {
			{
				resource = "Secret/apply-secret",
				environmentName = "dev",
				status = "applied",
				changedFields = {
					{ field = "data.PASSWORD" },
					{ field = "metadata.annotations.snapshot" },
					{ field = "stringData.TOKEN" },
				},
			},
		},
	})
end)

Test.k8s("Secret apply still updates the Kubernetes object", function(t)
	t.check("v1", "secrets", "dev", team:slug(), "apply-secret", {
		apiVersion = "v1",
		kind = "Secret",
		data = { PASSWORD = "TkVXX01BUktFUg==" },
		stringData = { TOKEN = "NEW_PLAINTEXT_MARKER" },
		metadata = {
			name = "apply-secret",
			namespace = team:slug(),
			annotations = { snapshot = "NEW_SNAPSHOT" },
		},
	})
end)

Test.rest("unchanged Secret apply has no changed fields", function(t)
	t.addHeader("x-user-email", user:email())
	t.send("POST", "/api/v1/teams/apply-secret-team/environments/dev/apply", update)
	t.check(200, {
		results = {
			{ resource = "Secret/apply-secret", environmentName = "dev", status = "applied" },
		},
	})
end)

Test.sql("Secret apply persists field paths but no values", function(t)
	t.queryRow([[
		SELECT
			resource_type,
			CONVERT_FROM(data, 'UTF8')::JSONB -> 'changedFields' AS changed_fields
		FROM activity_log_entries
		WHERE resource_name = 'apply-secret' AND action = 'UPDATED'
	]])
	t.check {
		resource_type = "SECRET",
		changed_fields = {
			{ field = "data.PASSWORD" },
			{ field = "metadata.annotations.snapshot" },
			{ field = "stringData.TOKEN" },
		},
	}
end)

Test.gql("other tenant users see Secret field names only", function(t)
	t.addHeader("x-user-email", outsider:email())
	t.query [[
		query {
			activityLog(first: 10, filter: { activityTypes: [SECRET_UPDATED] }) {
				nodes {
					__typename
					resourceType
					resourceName
					... on SecretUpdatedActivityLogEntry {
						data { updatedFields { field oldValue newValue } }
					}
				}
			}
		}
	]]
	t.check {
		data = {
			activityLog = {
				nodes = {
					{
						__typename = "SecretUpdatedActivityLogEntry",
						resourceType = "SECRET",
						resourceName = "apply-secret",
						data = {
							updatedFields = {
								{ field = "data.PASSWORD",                 oldValue = Null, newValue = Null },
								{ field = "metadata.annotations.snapshot", oldValue = Null, newValue = Null },
								{ field = "stringData.TOKEN",              oldValue = Null, newValue = Null },
							},
						},
					},
				},
			},
		},
	}
end)
