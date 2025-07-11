Helper.readK8sResources("k8s_resources/status")
local user = User.new("name", "auth@user.com", "sdf")
Team.new("slug-1", "purpose", "#slack_channel")

local function statusQuery(slug, env, appName, errorDetails)
	return string.format([[
		query {
			team(slug: "%s") {
				environment(name: "%s") {
					application(name: "%s") {
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

Test.gql("app with no errors", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(statusQuery("slug-1", "dev", "no-errors"))

	t.check {
		data = {
			team = {
				environment = {
					application = {
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

Test.gql("app with deprecated ingress", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(statusQuery("slug-1", "dev-gcp", "deprecated-ingress", [[
		...on WorkloadStatusDeprecatedIngress {
			ingress
		}
	]]))

	t.check {
		data = {
			team = {
				environment = {
					application = {
						status = {
							state = "NAIS",
							errors = {
								{
									__typename = "WorkloadStatusDeprecatedIngress",
									ingress = "https://error.dev-gcp.nais.io",
									level = "TODO",
								},
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("app with deprecated cloud sql instance", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(statusQuery("slug-1", "dev-gcp", "deprecated-cloud-sql", [[
		...on WorkloadStatusUnsupportedCloudSQLVersion {
			level
			version
		}
	]]))

	t.check {
		data = {
			team = {
				environment = {
					application = {
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

Test.gql("app with unsupported cloud sql instance", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(statusQuery("slug-1", "dev-gcp", "unsupported-cloud-sql", [[
		...on WorkloadStatusUnsupportedCloudSQLVersion {
			level
			version
		}
	]]))

	t.check {
		data = {
			team = {
				environment = {
					application = {
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

Test.gql("app with deprecated registry", function(t)
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
					application = {
						status = {
							state = "NOT_NAIS",
							errors = {
								{
									__typename = "WorkloadStatusDeprecatedRegistry",
									level = "ERROR",
									name = "app-name",
									registry = "navikt",
									repository = "",
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


Test.gql("app with naiserator invalid yaml", function(t)
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
					application = {
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


Test.gql("app with naiserator failed synchronization", function(t)
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
					application = {
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


-- Test.gql("app with failing netpols", function(t)
-- 	t.addHeader("x-user-email", user:email())

-- 	t.query(statusQuery("slug-1", "dev", "failed-netpol", [[
-- 		... on WorkloadStatusInboundNetwork {
-- 			policy {
-- 				targetWorkloadName
-- 				targetTeamSlug
-- 				mutual
-- 			}
-- 		}
-- 		... on WorkloadStatusOutboundNetwork {
-- 			policy {
-- 				targetWorkloadName
-- 				targetTeamSlug
-- 				mutual
-- 			}
-- 		}
-- 	]]))

-- 	t.check {
-- 		data = {
-- 			team = {
-- 				environment = {
-- 					application = {
-- 						status = {
-- 							state = "NOT_NAIS",
-- 							errors = {
-- 								{
-- 									__typename = "WorkloadStatusInboundNetwork",
-- 									level = "WARNING",
-- 									policy = {
-- 										mutual = false,
-- 										targetTeamSlug = "other-namespace",
-- 										targetWorkloadName = "other-app",
-- 									},
-- 								},
-- 								{
-- 									__typename = "WorkloadStatusOutboundNetwork",
-- 									level = "WARNING",
-- 									policy = {
-- 										mutual = false,
-- 										targetTeamSlug = "other-namespace",
-- 										targetWorkloadName = "other-app",
-- 									},
-- 								},
-- 								expectedMissingSBOM,
-- 							},
-- 						},
-- 					},
-- 				},
-- 			},
-- 		},
-- 	}
-- end)
