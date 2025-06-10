Helper.readK8sResources("k8s_resources/status")

local user = User.new("name", "auth@user.com", "sdf")
Team.new("slug-1", "purpose", "#slack_channel")


local function statusQuery(slug, env, appName, errorDetails)
	return string.format([[
		query {
			team(slug: "%s") {
				environment(name: "%s") {
					job(name: "%s") {
						status {
							state
							errors {
								__typename
								level
								%s
							}
						}
					}
				}
			}
		}
	]], slug, env, appName, errorDetails or "")
end

Test.gql("job with no errors", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(statusQuery("slug-1", "dev", "no-errors"))

	t.check {
		data = {
			team = {
				environment = {
					job = {
						status = {
							state = "NAIS",
							errors = {},
						},
					},
				},
			},
		},
	}
end)

Test.gql("job with deprecated cloud sql instance", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(statusQuery("slug-1", "dev-gcp", "jobname-1-deprecated-cloudsql", [[
		...on WorkloadStatusUnsupportedCloudSQLVersion {
			level
			version
		}
	]]))

	t.check {
		data = {
			team = {
				environment = {
					job = {
						status = {
							state = "NOT_NAIS",
							errors = {
								{
									__typename = "WorkloadStatusUnsupportedCloudSQLVersion",
									version = "POSTGRES_13",
									level = "WARNING",
								},
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("job with unsupported cloud sql instance", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(statusQuery("slug-1", "dev-gcp", "jobname-1-unsupported-cloudsql", [[
		...on WorkloadStatusUnsupportedCloudSQLVersion {
			level
			version
		}
	]]))

	t.check {
		data = {
			team = {
				environment = {
					job = {
						status = {
							state = "NOT_NAIS",
							errors = {
								{
									__typename = "WorkloadStatusUnsupportedCloudSQLVersion",
									version = "POSTGRES_12",
									level = "ERROR",
								},
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("job with deprecated registry", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(statusQuery("slug-1", "dev", "deprecated-registry", [[
		... on WorkloadStatusDeprecatedRegistry {
			registry
			repository
			name
			tag
		}
	]]))

	t.check {
		data = {
			team = {
				environment = {
					job = {
						status = {
							state = "NOT_NAIS",
							errors = {
								{
									__typename = "WorkloadStatusDeprecatedRegistry",
									level = "ERROR",
									name = "app-name",
									registry = "ghcr.io",
									repository = "navikt",
									tag = "latest",
								},
								{
									__typename = "WorkloadStatusVulnerable",
									level = "WARNING",
								},
							},
						},
					},
				},
			},
		},
	}
end)


Test.gql("job with naiserator invalid yaml", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(statusQuery("slug-1", "dev", "invalid-yaml", [[
		... on WorkloadStatusInvalidNaisYaml {
			detail
		}
	]]))

	t.check {
		data = {
			team = {
				environment = {
					job = {
						status = {
							state = "NOT_NAIS",
							errors = {
								{
									__typename = "WorkloadStatusInvalidNaisYaml",
									level = "ERROR",
									detail = "Human text from the operator, received from yaml",
								},
							},
						},
					},
				},
			},
		},
	}
end)


Test.gql("job with naiserator failed synchronization", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(statusQuery("slug-1", "dev", "failed-synchronization", [[
		... on WorkloadStatusSynchronizationFailing {
			detail
		}
	]]))

	t.check {
		data = {
			team = {
				environment = {
					job = {
						status = {
							state = "NOT_NAIS",
							errors = {
								{
									__typename = "WorkloadStatusSynchronizationFailing",
									level = "ERROR",
									detail = "Human text from the operator, received from yaml",
								},
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("job that failed, but is running", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(statusQuery("slug-1", "dev", "job-failed-running"))

	t.check {
		data = {
			team = {
				environment = {
					job = {
						status = {
							state = "NAIS",
							errors = {},
						},
					},
				},
			},
		},
	}
end)
