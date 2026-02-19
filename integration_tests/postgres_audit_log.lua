Helper.readK8sResources("k8s_resources/postgres_audit_log")

local user = User.new("authenticated", "postgres-audit-user@example.com", "postgres-audit-user-id")
local team = Team.new("audit-postgres-team", "Testing Postgres audit logging", "#audit-postgres")
team:addMember(user)
team:setEnvironmentGCPProjectID("dev-gcp", "nais-audit-project")

Test.gql("Postgres instance with audit logging enabled has audit URL", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "audit-postgres-team") {
				environment(name: "dev-gcp") {
					postgresInstance(name: "audit-enabled") {
						name
						audit {
							enabled
							url
						}
					}
				}
			}
		}
	]]

	t.check {
		data = {
			team = {
				environment = {
					postgresInstance = {
						name = "audit-enabled",
						audit = {
							enabled = true,
							url = Contains("console.cloud.google.com/logs"),
						},
					},
				},
			},
		},
	}
end)

Test.gql("Postgres audit URL contains expected components", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "audit-postgres-team") {
				environment(name: "dev-gcp") {
					postgresInstance(name: "audit-enabled") {
						audit {
							url
						}
					}
				}
			}
		}
	]]

	t.check {
		data = {
			team = {
				environment = {
					postgresInstance = {
						audit = {
							url = Contains("nais-audit-project%3Aaudit-enabled"),
						},
					},
				},
			},
		},
	}
end)
