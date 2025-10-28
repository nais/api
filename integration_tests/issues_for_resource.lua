Helper.readK8sResources("k8s_resources/issues")
local user = User.new("name", "auth@user.com", "sdf")
Team.new("myteam", "purpose", "#slack_channel")
Team.new("sortteam", "purpose", "#slack_channel")
local checker = IssueChecker.new()

Test.gql("Workloads with issues", function(t)
	checker:runChecks()
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "myteam") {
				environment(name: "dev-gcp") {
			  		application(name: "deprecated-app") {
						issues {
							nodes {
								__typename
								severity
								message
								... on DeprecatedIngressIssue {
									ingresses
								}
							}
				  		}
					}
					job(name: "deprecated-job") {
						issues {
							nodes {
								__typename
								severity
								message
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
				environment = {
					application = {
						issues = {
							nodes = {
								{
									__typename = "DeprecatedRegistryIssue",
									message = "Image 'deprecated.dev/nais/navikt/app-name:latest' is using a deprecated registry",
									severity = "WARNING",
								},
								{
									__typename = "DeprecatedIngressIssue",
									message = "Application is using deprecated ingresses: [https://foo.dev-gcp.nais.io https://bar.dev-gcp.nais.io]",
									severity = "TODO",
									ingresses = { "https://foo.dev-gcp.nais.io", "https://bar.dev-gcp.nais.io" },
								},
							},
						},
					},
					job = {
						issues = {
							nodes = {
								{
									__typename = "DeprecatedRegistryIssue",
									message = "Image 'ghcr.io/navikt/app-name:latest' is using a deprecated registry",
									severity = "WARNING",
								},
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("SqlInstance with issues", function(t)
	checker:runChecks()
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "myteam") {
				environment(name: "dev-gcp") {
			  		sqlInstance(name: "stopped") {
						issues {
							nodes {
								__typename
								severity
								message
								... on SqlInstanceStateIssue {
									state
								}
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
				environment = {
					sqlInstance = {
						issues = {
							nodes = {
								{
									__typename = "SqlInstanceStateIssue",
									message = "The instance has been stopped.",
									severity = "CRITICAL",
									state = "STOPPED",
								},
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("Valkey with issues", function(t)
	checker:runChecks()
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "myteam") {
				environment(name: "dev-gcp") {
			  		valkey(name: "valkey-myteam-name") {
						issues {
							nodes {
								__typename
								severity
								message
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
				environment = {
					valkey = {
						issues = {
							nodes = {
								{
									__typename = "ValkeyIssue",
									message = "Your valkey service valkey-myteam-name reports: error message from aiven",
									severity = "CRITICAL",
								},
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("Opensearch with issues", function(t)
	checker:runChecks()
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "myteam") {
				environment(name: "dev-gcp") {
			  		openSearch(name: "opensearch-myteam-name") {
						issues {
							nodes {
								__typename
								severity
								message
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
				environment = {
					openSearch = {
						issues = {
							nodes = {
								{
									__typename = "OpenSearchIssue",
									message = "Your opensearch service opensearch-myteam-name reports: error message from aiven",
									severity = "CRITICAL",
								},
							},
						},
					},
				},
			},
		},
	}
end)

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

Test.gql("Jobs sorted by issues", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "sortteam") {
				jobs(
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
				jobs = {
					nodes = {
						{
							name = "job-warning",
							issues = {
								nodes = {
									{ severity = "WARNING" },
								},
							},
						},
						{
							name = "job-no-issues",
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

Test.gql("OpenSearches sorted by issues", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "sortteam") {
				openSearches(
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
				openSearches = {
					nodes = {
						{
							name = "opensearch-sortteam-critical",
							issues = {
								nodes = {
									{ severity = "CRITICAL" },
								},
							},
						},
						{
							name = "opensearch-sortteam-running",
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

Test.gql("Valkeys sorted by issues", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "sortteam") {
				valkeys(
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
				valkeys = {
					nodes = {
						{
							name = "valkey-sortteam-critical",
							issues = {
								nodes = {
									{ severity = "CRITICAL" },
								},
							},
						},
						{
							name = "valkey-sortteam-running",
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

Test.gql("SqlInstances sorted by issues", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "sortteam") {
				sqlInstances(
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
				sqlInstances = {
					nodes = {
						{
							name = "stopped",
							issues = {
								nodes = {
									{ severity = "CRITICAL" },
								},
							},
						},
						{
							name = "running",
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
