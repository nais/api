-- Currently, there's no way to provide dependencytrack data, so we consider missing sbom
-- as valid empty condition
local expectedMissingSBOM = { __typename = "WorkloadStatusMissingSBOM", level = "TODO" }

Helper.readK8sResources("k8s_resources/status")

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
	t.query(statusQuery("slug-1", "dev", "no-errors"))

	t.check {
		data = {
			team = {
				environment = {
					job = {
						status = {
							state = "NAIS",
							errors = { expectedMissingSBOM },
						},
					},
				},
			},
		},
	}
end)

Test.gql("job with deprecated registry", function(t)
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
							state = "NAIS",
							errors = {
								expectedMissingSBOM,
								{
									__typename = "WorkloadStatusDeprecatedRegistry",
									level = "WARNING",
									name = "app-name",
									registry = "ghcr.io",
									repository = "navikt",
									tag = "latest",
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
								expectedMissingSBOM,
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
								expectedMissingSBOM,
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


Test.gql("job with failing netpols", function(t)
	t.query(statusQuery("slug-1", "dev", "failed-netpol", [[
		... on WorkloadStatusInboundNetwork {
			policy {
				targetWorkloadName
				targetTeamSlug
				mutual
			}
		}
		... on WorkloadStatusOutboundNetwork {
			policy {
				targetWorkloadName
				targetTeamSlug
				mutual
			}
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
								expectedMissingSBOM,
								{
									__typename = "WorkloadStatusInboundNetwork",
									level = "WARNING",
									policy = {
										mutual = false,
										targetTeamSlug = "other-namespace",
										targetWorkloadName = "other-app",
									},
								},
								{
									__typename = "WorkloadStatusOutboundNetwork",
									level = "WARNING",
									policy = {
										mutual = false,
										targetTeamSlug = "other-namespace",
										targetWorkloadName = "other-app",
									},
								},
							},
						},
					},
				},
			},
		},
	}
end)
