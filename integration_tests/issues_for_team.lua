Helper.readK8sResources("k8s_resources/issues")
local user = User.new("name", "auth@user.com", "sdf")
Team.new("myteam", "purpose", "#slack_channel")
Team.new("sortteam", "purpose", "#slack_channel")
local checker = IssueChecker.new()
checker:runChecks()

Test.gql("Applications sorted by issues", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "sortteam") {
				applications(
					orderBy: {
						field: ISSUES,
						direction: DESC
					}
				) {
					nodes {
						name
						issues {
							nodes {
								severity
							}
						}
					}
				}
			}
		}
	]]

	t.check {
		data = {
			team = {
				applications = {
					nodes = {
						{
							name = "app-critical",
							issues = {
								nodes = {
									{ severity = "CRITICAL" },
								},
							},
						},
						{
							name = "app-warning-todo",
							issues = {
								nodes = {
									{ severity = "WARNING" },
									{ severity = "TODO" },
								},
							},
						},
						{
							name = "app-warning",
							issues = {
								nodes = {
									{ severity = "WARNING" },
								},
							},
						},
						{
							name = "app-todo",
							issues = {
								nodes = {
									{ severity = "TODO" },
								},
							},
						},
						{
							name = "app-no-issues",
							issues = {
								nodes = {},
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("DeprecatedIngressIssue", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "myteam") {
				issues(
					filter: {
						issueType: DEPRECATED_INGRESS
					}
				) {
					nodes {
						__typename
						severity
						message
						... on DeprecatedIngressIssue {
							ingresses
							application {
								name
							}
						}
					}
				}
			}
		}
	]]

	t.check {
		data = {
			team = {
				issues = {
					nodes = {
						{
							__typename = "DeprecatedIngressIssue",
							application = {
								name = "deprecated-app",
							},
							message = "Application is using deprecated ingresses: [https://foo.dev-gcp.nais.io https://bar.dev-gcp.nais.io]",
							severity = "TODO",
							ingresses = { "https://foo.dev-gcp.nais.io", "https://bar.dev-gcp.nais.io" },
						},
					},
				},
			},
		},
	}
end)

Test.gql("DeprecatedRegistryIssue", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "myteam") {
				issues(
					filter: {
						issueType: DEPRECATED_REGISTRY
					},
					orderBy: {
						field: RESOURCE_NAME,
						direction: ASC
					}
				) {
					nodes {
						__typename
						severity
						message
						... on DeprecatedRegistryIssue {
							workload {
								name
							}
						}
					}
				}
			}
		}
	]]

	t.check {
		data = {
			team = {
				issues = {
					nodes = {
						{
							__typename = "DeprecatedRegistryIssue",
							message = "Image 'deprecated.dev/nais/navikt/app-name:latest' is using a deprecated registry",
							severity = "WARNING",
							workload = {
								name = "deprecated-app",
							},
						},
						{
							__typename = "DeprecatedRegistryIssue",
							message = "Image 'ghcr.io/navikt/app-name:latest' is using a deprecated registry",
							severity = "WARNING",
							workload = {
								name = "deprecated-job",
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("FailedJobRunsIssue", function(t)
	checker:runChecks()
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "myteam") {
				issues(
					filter: {
						issueType: FAILED_JOB_RUNS
					},
				) {
					nodes {
						__typename
						severity
						message
						... on FailedJobRunsIssue {
							job {
								name
							}
						}
					}
				}
			}
		}
	]]

	t.check {
		data = {
			team = {
				issues = {
					nodes = {
						--	{
						--		__typename = "FailedJobRunsIssue",
						--		message = "TODO",
						--		severity = "WARNING",
						--		job = {
						--			name = "deprecated-job",
						--		},
						--	},
					},
				},
			},
		},
	}
end)
Test.gql("FailedSynchronizationIssue", function(t)
	checker:runChecks()
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "myteam") {
				issues(
					filter: {
						issueType: FAILED_SYNCHRONIZATION
					},
				) {
					nodes {
						__typename
						severity
						message
						... on FailedSynchronizationIssue {
							workload {
								name
							}
						}
					}
				}
			}
		}
	]]

	t.check {
		data = {
			team = {
				issues = {
					nodes = {
						{
							__typename = "FailedSynchronizationIssue",
							message = "Human text from the operator, received from yaml",
							severity = "WARNING",
							workload = {
								name = "failed-synchronization",
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("InvalidSpecIssue", function(t)
	checker:runChecks()
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "myteam") {
				issues(
					filter: {
						issueType: INVALID_SPEC
					},
					orderBy: {
						field: RESOURCE_NAME
						direction:ASC
					}
				) {
					nodes {
						__typename
						severity
						message
						... on InvalidSpecIssue {
							workload {
								__typename
								name
							}
						}
					}
				}
			}
		}
	]]

	t.check {
		data = {
			team = {
				issues = {
					nodes = {
						{
							__typename = "InvalidSpecIssue",
							message = "Human readable text from the operator",
							severity = "CRITICAL",
							workload = {
								__typename = "Application",
								name = "app-failed-generate",
							},
						},
						{
							__typename = "InvalidSpecIssue",
							message = "Human readable text from the operator",
							severity = "CRITICAL",
							workload = {
								__typename = "Job",
								name = "job-failed-generate",
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("NoRunningInstancesIssue", function(t)
	checker:runChecks()
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "myteam") {
				issues(
					filter: {
						issueType: NO_RUNNING_INSTANCES
					},
					orderBy: {
						field: RESOURCE_NAME
						direction: ASC
					}

				) {
					nodes {
						__typename
						severity
						message
						... on NoRunningInstancesIssue {
							workload {
								name
							}
						}
					}
				}
			}
		}
	]]

	t.check {
		data = {
			team = {
				issues = {
					nodes = {
						{
							__typename = "NoRunningInstancesIssue",
							message = "Application has no running instances",
							severity = "CRITICAL",
							workload = {
								name = "failed-synchronization",
							},
						},
						{
							__typename = "NoRunningInstancesIssue",
							message = "Application has no running instances",
							severity = "CRITICAL",
							workload = {
								name = "missing-instances",
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("OpenSearchIssue", function(t)
	checker:runChecks()
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "myteam") {
				issues(
					filter: {
						issueType: OPENSEARCH,
						environments: ["dev-gcp"]
					},
				) {
					nodes {
						__typename
						severity
						message
						... on  OpenSearchIssue {
							event
							openSearch {
								name
							}
						}
					}
				}
			}
		}
	]]

	t.check {
		data = {
			team = {
				issues = {
					nodes = {
						{
							__typename = "OpenSearchIssue",
							message = "Your opensearch service opensearch-myteam-name reports: error message from aiven",
							event = "error message from aiven",
							openSearch = {
								name = "opensearch-myteam-name",
							},
							severity = "CRITICAL",
						},
					},
				},
			},
		},
	}
end)

Test.gql("SqlInstanceStateIssue", function(t)
	checker:runChecks()
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "myteam") {
				issues(
					filter: {
						issueType: SQLINSTANCE_STATE
					},
					orderBy: {
						field: RESOURCE_NAME
						direction: ASC
					}
				) {
					nodes {
						__typename
						severity
						message
						... on  SqlInstanceStateIssue {
							state
							sqlInstance {
								name
							}
						}
					}
				}
			}
		}
	]]

	t.check {
		data = {
			team = {
				issues = {
					nodes = {
						{
							__typename = "SqlInstanceStateIssue",
							sqlInstance = {
								name = "maintenance",
							},
							severity = "CRITICAL",
							state = "MAINTENANCE",
							message = "The instance is down for maintenance.",
						},
						{
							__typename = "SqlInstanceStateIssue",
							message = "The instance has been stopped.",
							severity = "CRITICAL",
							sqlInstance = {
								name = "stopped",
							},
							state = "STOPPED",
						},
					},
				},
			},
		},
	}
end)

Test.gql("SqlInstanceVersionIssue", function(t)
	checker:runChecks()
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "myteam") {
				issues(
					filter: {
						issueType: SQLINSTANCE_VERSION
					},
				) {
					nodes {
						__typename
						severity
						message
						... on SqlInstanceVersionIssue {
							sqlInstance {
								name
							}
						}
					}
				}
			}
		}
	]]

	t.check {
		data = {
			team = {
				issues = {
					nodes = {
						{
							__typename = "SqlInstanceVersionIssue",
							message = "The instance is running a deprecated version of PostgreSQL: POSTGRES_12",
							severity = "WARNING",
							sqlInstance = {
								name = "deprecated",
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("VulnerableImageIssue", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "myteam") {
				issues(
					filter: {
						issueType: VULNERABLE_IMAGE
					},
				) {
					nodes {
						__typename
						severity
						message
						... on VulnerableImageIssue {
							critical
							riskScore
							workload {
								name
							}
						}
					}
				}
			}
		}
	]]

	t.check {
		data = {
			team = {
				issues = {
					nodes = {
						{
							__typename = "VulnerableImageIssue",
							message = "Image 'vulnerable-image' has 5 critical vulnerabilities and a risk score of 250",
							severity = "WARNING",
							critical = 5,
							riskScore = 250,
							workload = {
								name = "vulnerable",
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("MissingSbomIssue", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "myteam") {
				issues(
					filter: {
						issueType: MISSING_SBOM
					},
				) {
					nodes {
						__typename
						severity
						message
						... on MissingSbomIssue {
							workload {
								name
							}
						}
					}
				}
			}
		}
	]]

	t.check {
		data = {
			team = {
				issues = {
					nodes = {
						{
							__typename = "MissingSbomIssue",
							message = "Image 'missing-sbom-image:tag1' is missing a Software Bill of Materials (SBOM)",
							severity = "WARNING",
							workload = {
								name = "missing-sbom",
							},
						},
					},
				},
			},
		},
	}
end)
