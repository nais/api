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
								state
								analysisTrail {
									state
									suppressed
									comments {
										nodes {
											comment
											onBehalfOf
											state
											suppressed
											timestamp
										}
									}
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
										state = NotNull(),
										analysisTrail = {
											state = NotNull(),
											suppressed = NotNull(),
											comments = NotNull(),
										},
									},
									{
										description = NotNull(),
										identifier = NotNull(),
										package = NotNull(),
										severity = NotNull(),
										state = NotNull(),
										analysisTrail = {
											state = NotNull(),
											suppressed = NotNull(),
											comments = NotNull(),
										},
									},
									{
										description = NotNull(),
										identifier = NotNull(),
										package = NotNull(),
										severity = NotNull(),
										state = NotNull(),
										analysisTrail = {
											state = NotNull(),
											suppressed = NotNull(),
											comments = NotNull(),
										},
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
