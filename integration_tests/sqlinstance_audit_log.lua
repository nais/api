Helper.readK8sResources("k8s_resources/audit_log")

local user = User.new("authenticated", "audit-user@example.com", "audit-user-id")
local team = Team.new("audit-team", "Testing SQL audit logging", "#audit-team")

Test.gql("SQL Instance with audit logging enabled", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "audit-team") {
				environment(name: "dev") {
					sqlInstance(name: "audit-enabled") {
						name
						auditLog {
							logUrl
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
					sqlInstance = {
						name = "audit-enabled",
						auditLog = {
							logUrl = Contains("console.cloud.google.com/logs"),
						},
					},
				},
			},
		},
	}
end)

Test.gql("SQL Instance with audit logging disabled", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "audit-team") {
				environment(name: "dev") {
					sqlInstance(name: "audit-disabled") {
						name
						auditLog {
							logUrl
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
					sqlInstance = {
						name = "audit-disabled",
						auditLog = Null,
					},
				},
			},
		},
	}
end)

Test.gql("SQL Instance without audit flag", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "audit-team") {
				environment(name: "dev") {
					sqlInstance(name: "no-audit-flag") {
						name
						auditLog {
							logUrl
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
					sqlInstance = {
						name = "no-audit-flag",
						auditLog = Null,
					},
				},
			},
		},
	}
end)

Test.gql("Audit log URL contains correct components", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "audit-team") {
				environment(name: "dev") {
					sqlInstance(name: "audit-enabled") {
						auditLog {
							logUrl
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
					sqlInstance = {
						auditLog = {
							-- Check that the URL contains the expected components
							logUrl = Contains("nais-dev-2e7b%3Aaudit-enabled"), -- URL encoded database_id
						},
					},
				},
			},
		},
	}
end)

Test.gql("Audit log URL contains team storage scope", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "audit-team") {
				environment(name: "dev") {
					sqlInstance(name: "audit-enabled") {
						auditLog {
							logUrl
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
					sqlInstance = {
						auditLog = {
							-- Check that the URL contains the team in the storage scope
							logUrl = Contains("audit-team"),
						},
					},
				},
			},
		},
	}
end)

Test.gql("Multiple SQL instances with different audit configurations", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "audit-team") {
				environment(name: "dev") {
					enabled: sqlInstance(name: "audit-enabled") {
						auditLog {
							logUrl
						}
					}
					disabled: sqlInstance(name: "audit-disabled") {
						auditLog {
							logUrl
						}
					}
					noFlag: sqlInstance(name: "no-audit-flag") {
						auditLog {
							logUrl
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
					enabled = {
						auditLog = {
							logUrl = Contains("console.cloud.google.com/logs"),
						},
					},
					disabled = {
						auditLog = Null,
					},
					noFlag = {
						auditLog = Null,
					},
				},
			},
		},
	}
end)
