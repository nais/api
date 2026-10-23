Helper.readK8sResources("k8s_resources/vulnerability")
local team = Team.new("slug-1", "purpose", "#channel")
local user = User.new("authenticated", "authenticated@example.com", "some-id")
local reanalysisMember = User.new("reanalysis-member", "reanalysis-member@example.com", "reanalysis-member")
team:addMember(reanalysisMember)
local reanalysisAdmin = User.new("reanalysis-admin", "reanalysis-admin@example.com", "reanalysis-admin")
reanalysisAdmin:admin(true)
local reanalysisGlobalMember = User.new("reanalysis-global-member", "reanalysis-global-member@example.com",
	"reanalysis-global-member")
Helper.SQLExec("INSERT INTO user_roles (user_id, role_name) VALUES ($1, $2)", reanalysisGlobalMember:id(), "Team member")
local reanalysisServiceAccount = ServiceAccount.new("reanalysis-service-account", team:slug())
reanalysisServiceAccount:assignRole("Team member")

Test.gql("Reanalyze an application using WorkloadType", function(t)
	t.addHeader("x-user-email", reanalysisMember:email())
	t.query [[
		mutation {
			reanalyzeVulnerabilities(input: {
				teamSlug: "slug-1"
				environmentName: "dev"
				workloadName: "app-with-vulnerabilities"
				workloadType: APPLICATION
			}) {
				numImages
				requestedAt
			}
		}
	]]
	t.check {
		data = {
			reanalyzeVulnerabilities = {
				numImages = 0,
				requestedAt = Null,
			},
		},
	}
end)

Test.gql("Reject reanalysis from a non-member", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		mutation {
			reanalyzeVulnerabilities(input: { teamSlug: "slug-1", environmentName: "dev" }) {
				numImages
			}
		}
	]]
	t.check {
		data = Null,
		errors = {
			{
				locations = NotNull(),
				message = Contains("You are authenticated"),
				path = { "reanalyzeVulnerabilities" },
			},
		},
	}
end)

for _, actor in ipairs({
	{ name = "service account with vulnerability:update",   header = "Authorization", value = "Bearer " .. reanalysisServiceAccount:token() },
	{ name = "non-member admin",                            header = "x-user-email",  value = reanalysisAdmin:email() },
	{ name = "non-member with global vulnerability:update", header = "x-user-email",  value = reanalysisGlobalMember:email() },
}) do
	Test.gql("Reject reanalysis from " .. actor.name, function(t)
		t.addHeader(actor.header, actor.value)
		t.query [[
			mutation {
				reanalyzeVulnerabilities(input: { teamSlug: "slug-1", environmentName: "dev" }) {
					numImages
				}
			}
		]]
		t.check {
			data = Null,
			errors = {
				{
					locations = NotNull(),
					message = Contains("You are authenticated"),
					path = { "reanalyzeVulnerabilities" },
				},
			},
		}
	end)
end

for _, selector in ipairs({
	{ name = "type without name", fields = 'workloadType: APPLICATION',                  message = "workloadType requires workloadName" },
	{ name = "name without type", fields = 'workloadName: "app-with-vulnerabilities"',   message = "workloadName and workloadType must both be set" },
	{ name = "empty name",        fields = 'workloadName: "" workloadType: APPLICATION', message = "workloadName and workloadType must both be set" },
}) do
	Test.gql("Reject reanalysis with " .. selector.name, function(t)
		t.addHeader("x-user-email", reanalysisMember:email())
		t.query(string.format([[
			mutation {
				reanalyzeVulnerabilities(input: {
					teamSlug: "slug-1"
					environmentName: "dev"
					%s
				}) {
					numImages
				}
			}
		]], selector.fields))
		t.check {
			data = Null,
			errors = {
				{
					locations = NotNull(),
					message = Contains(selector.message),
					path = { "reanalyzeVulnerabilities" },
				},
			},
		}
	end)
end

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
									countsBySeverity {
										critical
										high
										medium
										low
										unassigned
									}
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
							{ date = NotNull(), summary = { total = NotNull(), riskScore = NotNull(), critical = NotNull(), high = NotNull(), medium = NotNull(), low = NotNull(), unassigned = NotNull(), countsBySeverity = { critical = NotNull(), high = NotNull(), medium = NotNull(), low = NotNull(), unassigned = NotNull() } } },
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
					  tag
					  digest
					  vulnerabilities(first: 10) {
					  	nodes {
					  		identifier
					  		riskAssessment {
					  			priority
					  		}
					  	}
					  }
					  hasSBOM
				  sbom {
					status
					processingStartedAt
				  }
					  vulnerabilitySummary{
						total
						countsBySeverity {
							critical
							high
							medium
							low
							unassigned
						}
						countsByPriority {
							urgent
							knownExploited
							highRisk
							elevatedRisk
							monitor
						}
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
								tag = "latest",
								digest = "sha256:deadbeef",
								vulnerabilities = {
									nodes = {
										{
											identifier = NotNull(),
											riskAssessment = {
												priority = NotNull(),
											},
										},
										{
											identifier = NotNull(),
											riskAssessment = {
												priority = NotNull(),
											},
										},
										{
											identifier = NotNull(),
											riskAssessment = {
												priority = NotNull(),
											},
										},
									},
								},
								hasSBOM = true,
								sbom = {
									status = "READY",
									processingStartedAt = Ignore(),
								},
								vulnerabilitySummary = {
									total = NotNull(),
									countsBySeverity = {
										critical = NotNull(),
										high = NotNull(),
										medium = NotNull(),
										low = NotNull(),
										unassigned = NotNull(),
									},
									countsByPriority = {
										urgent = 0,
										knownExploited = NotNull(),
										highRisk = NotNull(),
										elevatedRisk = NotNull(),
										monitor = NotNull(),
									},
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
				countsBySeverity {
					critical
					high
					medium
					low
					unassigned
				}
				countsByPriority {
					urgent
					knownExploited
					highRisk
					elevatedRisk
					monitor
				}
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
				countsBySeverity = {
					critical = NotNull(),
					high = NotNull(),
					medium = NotNull(),
					low = NotNull(),
					unassigned = NotNull(),
				},
				countsByPriority = {
					urgent = 0,
					knownExploited = NotNull(),
					highRisk = NotNull(),
					elevatedRisk = NotNull(),
					monitor = NotNull(),
				},
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

Test.gql("List vulnerability summaries across teams", function(t)
	t.addHeader("x-user-email", user:email())
	t.query([[
		{
			vulnerabilitySummaries(first: 1) {
				pageInfo { totalCount }
				nodes { id }
			}
		}
	]])

	t.check {
		data = {
			vulnerabilitySummaries = {
				pageInfo = { totalCount = NotNull() },
				nodes = { { id = NotNull() } },
			},
		},
	}
end)

Test.gql("Filter team workloads by SBOM status", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		{
			team(slug: "%s") {
				workloads(first: 10, filter: { sbomStatus: NO_SBOM }) {
					pageInfo { totalCount }
					nodes { name }
				}
			}
		}
	]], team:slug()))

	t.check {
		data = {
			team = {
				workloads = {
					pageInfo = { totalCount = 0 },
					nodes = {},
				},
			},
		},
	}
end)

Test.gql("Legacy vulnerability fields remain queryable", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		{
			team(slug: "%s") {
				environment(name: "dev") {
					workload(name: "app-with-vulnerabilities") {
						image {
							vulnerabilities(first: 1) {
								nodes {
									priority
									cvssScore
									epssScore
									epssPercentile
									hasKevEntry
									knownRansomwareUse
									fixVersion
									riskAssessment { priority }
									remediation { fixVersion }
								}
							}
						}
					}
				}
			}
			cve(identifier: "CVE-2024-12345") {
				priority
				epssScore
				epssPercentile
				hasKevEntry
				knownRansomwareUse
				riskAssessment { priority }
			}
		}
	]], team:slug()))

	t.check {
		data = {
			team = {
				environment = {
					workload = {
						image = {
							vulnerabilities = {
								nodes = {
									{
										priority = NotNull(),
										cvssScore = Ignore(),
										epssScore = Ignore(),
										epssPercentile = Ignore(),
										hasKevEntry = NotNull(),
										knownRansomwareUse = NotNull(),
										fixVersion = Ignore(),
										riskAssessment = { priority = NotNull() },
										remediation = { fixVersion = Ignore() },
									},
								},
							},
						},
					},
				},
			},
			cve = {
				priority = NotNull(),
				epssScore = Ignore(),
				epssPercentile = Ignore(),
				hasKevEntry = NotNull(),
				knownRansomwareUse = NotNull(),
				riskAssessment = { priority = NotNull() },
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
				countsBySeverity {
					critical
					high
					medium
					low
					unassigned
				}
				countsByPriority {
					urgent
					knownExploited
					highRisk
					elevatedRisk
					monitor
				}
				highWorkloadCount
				elevatedWorkloadCount
				monitorWorkloadCount
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
					countsBySeverity = {
						critical = NotNull(),
						high = NotNull(),
						medium = NotNull(),
						low = NotNull(),
						unassigned = NotNull(),
					},
					countsByPriority = {
						urgent = 0,
						knownExploited = NotNull(),
						highRisk = NotNull(),
						elevatedRisk = NotNull(),
						monitor = NotNull(),
					},
					highWorkloadCount = NotNull(),
					elevatedWorkloadCount = NotNull(),
					monitorWorkloadCount = NotNull(),
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

Test.gql("Workload counts per priority are counted per team, not per finding", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(string.format([[
		{
			team(slug: "%s") {
			  vulnerabilitySummary{
				highWorkloadCount
				elevatedWorkloadCount
				monitorWorkloadCount
			  }
			}
		}
	]], team:slug()))

	t.check {
		data = {
			team = {
				vulnerabilitySummary = {
					highWorkloadCount = 1,
					elevatedWorkloadCount = 0,
					monitorWorkloadCount = 0,
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
				riskAssessment {
					priority
				}
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
				riskAssessment = {
					priority = NotNull(),
				},
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
