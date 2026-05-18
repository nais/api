Helper.readK8sResources("k8s_resources/vulnerability")
local team = Team.new("slug-1", "purpose", "#channel")
local user = User.new("authenticated", "authenticated@example.com", "some-id")

Test.gql("List vulnerability history for image", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		{
			team(slug: "%s") {
				environment(name: "%s") {
					workload(name: "%s") {
						imageVulnerabilityHistory(from: "%s") {
							samples {
								summary {
									riskScore
									total
									critical
									high
									medium
									low
									unassigned
								}
								date
							}
						}
					}
				}
			}
		}
	]], team:slug(), "dev", "app-with-vulnerabilities", os.date("%Y-%m-%d")))

	t.check {
		data = {
			team = {
				environment = {
					workload = {
						imageVulnerabilityHistory = { samples = {
							{ date = NotNull(), summary = { total = NotNull(), riskScore = NotNull(), critical = NotNull(), high = NotNull(), medium = NotNull(), low = NotNull(), unassigned = NotNull() } },
						} },
					},
				},
			},
		},
	}
end)


Test.gql("List vulnerability summaries for team", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		{
			team(slug: "%s") {
				workloads{
				  nodes{
					image{
					  name
					  hasSBOM
				  sbom {
					status
					processingStartedAt
				  }
					  vulnerabilitySummary{
						total
						critical
						high
						medium
						low
						unassigned
						riskScore
					  }
					}
				  }
				}
			}
		}
	]], team:slug()))

	t.check {
		data = {
			team = {
				workloads = {
					nodes = {
						{
							image = {
								name = "europe-north1-docker.pkg.dev/nais/navikt/app-name",
								hasSBOM = true,
								sbom = {
									status = "READY",
									processingStartedAt = Ignore(),
								},
								vulnerabilitySummary = {
									total = NotNull(),
									critical = NotNull(),
									high = NotNull(),
									medium = NotNull(),
									low = NotNull(),
									unassigned = NotNull(),
									riskScore = NotNull(),
								},
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("Get vulnerability summary for tenant", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		{
			vulnerabilitySummary{
				critical
				high
				medium
				low
				unassigned
				riskScore
				sbomCount
				coverage
			}
		}
	]]))

	t.check {
		data = {
			vulnerabilitySummary = {
				critical = NotNull(),
				high = NotNull(),
				medium = NotNull(),
				low = NotNull(),
				unassigned = NotNull(),
				riskScore = NotNull(),
				sbomCount = NotNull(),
				coverage = NotNull(),
			},
		},
	}
end)

Test.gql("Get vulnerability summary for team", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		{
			team(slug: "%s") {
			  vulnerabilitySummary{
				critical
				high
				medium
				low
				unassigned
				riskScore
				coverage
			  }
			}
		}
	]], team:slug()))

	t.check {
		data = {
			team = {
				vulnerabilitySummary = {
					critical = NotNull(),
					high = NotNull(),
					medium = NotNull(),
					low = NotNull(),
					unassigned = NotNull(),
					riskScore = NotNull(),
					coverage = NotNull(),
				},
			},
		},
	}
end)

Test.gql("Get CVE workloads without filter", function(t)
	t.addHeader("x-user-email", user:email())
	t.query([[
		{
			cve(identifier: "CVE-2024-12345") {
				identifier
				workloads(first: 10) {
					pageInfo {
						totalCount
					}
					nodes {
						workload {
							name
						}
					}
				}
			}
		}
	]])

	t.check {
		data = {
			cve = {
				identifier = "CVE-2024-12345",
				workloads = {
					pageInfo = { totalCount = 1 },
					nodes = {
						{ workload = { name = "app-with-vulnerabilities" } },
					},
				},
			},
		},
	}
end)

Test.gql("Get CVE workloads filtered by teamSlugs", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		{
			cve(identifier: "CVE-2024-12345") {
				identifier
				workloads(first: 10, filter: { teamSlugs: ["%s"] }) {
					pageInfo {
						totalCount
					}
					nodes {
						workload {
							name
						}
					}
				}
			}
		}
	]], team:slug()))

	t.check {
		data = {
			cve = {
				identifier = "CVE-2024-12345",
				workloads = {
					pageInfo = { totalCount = 1 },
					nodes = {
						{ workload = { name = "app-with-vulnerabilities" } },
					},
				},
			},
		},
	}
end)

Test.gql("Get WorkloadWithVulnerability node by id", function(t)
	t.addHeader("x-user-email", user:email())
	t.query([[
		{
			cve(identifier: "CVE-2024-12345") {
				workloads(first: 1) {
					nodes {
						id
					}
				}
			}
		}
	]])

	t.check {
		data = {
			cve = {
				workloads = {
					nodes = {
						{ id = Save("workload_with_vulnerability_id") },
					},
				},
			},
		},
	}

	local id = State.workload_with_vulnerability_id

	t.query(string.format([[
		{
			node(id: "%s") {
				id
				... on WorkloadWithVulnerability {
					workload {
						name
					}
					vulnerability {
						identifier
					}
				}
			}
		}
	]], id))

	t.check {
		data = {
			node = {
				id = id,
				workload = {
					name = "app-with-vulnerabilities",
				},
				vulnerability = {
					identifier = "CVE-2024-12345",
				},
			},
		},
	}
end)

Test.gql("List vulnerabilities for image", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		{
			team(slug: "%s") {
			slug
			environment(name: "%s") {
				environment {
					name
				}
				workload(name: "%s") {
					image {
						vulnerabilities(first: 10) {
							nodes {
								description
								identifier
								package
								severity
								suppression {
									reason
									state
								}
							}
						}
					}
				}
			}
		}
	}
	]], team:slug(), "dev", "app-with-vulnerabilities"))

	t.check {
		data = {
			team = {
				slug = team:slug(),
				environment = {
					environment = {
						name = "dev",
					},
					workload = {
						image = {
							vulnerabilities = {
								nodes = {
									{
										description = NotNull(),
										identifier = NotNull(),
										package = NotNull(),
										severity = NotNull(),
										suppression = Null,
									},
									{
										description = NotNull(),
										identifier = NotNull(),
										package = NotNull(),
										severity = NotNull(),
										suppression = Null,
									},
									{
										description = NotNull(),
										identifier = NotNull(),
										package = NotNull(),
										severity = NotNull(),
										suppression = Null,
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
