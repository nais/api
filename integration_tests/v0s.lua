Config.V0s = true
local user = User.new()
Team.new("v0s-team", "Vulnerability migration", "#v0s")
Helper.readK8sResources("k8s_resources/v0s")

Test.gql("Tenant and team summaries include full inventory independently of finding filters", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{
		vulnerabilityFindings(first: 1) {
			summary {
				findingCount uniqueVulnerabilityCount affectedWorkloadCount
				knownExploitedVulnerabilityCount knownExploitedWorkloadCount
				countsBySeverity { critical high medium low unknown }
				coverage { totalWorkloads scanned pending noSbom failed unresolvedImage }
			}
		}
		team(slug: "v0s-team") {
			vulnerabilityFindings(first: 1) {
				summary {
					findingCount uniqueVulnerabilityCount affectedWorkloadCount
					coverage { totalWorkloads scanned pending }
				}
			}
			environment(name: "dev") {
				vulnerabilityFindings(filter: {onlyWithFix: true}) {
					summary {
						findingCount uniqueVulnerabilityCount affectedWorkloadCount
						knownExploitedVulnerabilityCount knownExploitedWorkloadCount
						countsBySeverity { high critical }
						coverage { totalWorkloads scanned pending }
					}
				}
			}
			empty: vulnerabilityFindings(filter: {packageType: "unknown"}) {
				summary {
					findingCount uniqueVulnerabilityCount affectedWorkloadCount
					coverage { totalWorkloads scanned pending }
				}
			}
		}
	}]]
	t.check { data = {
		vulnerabilityFindings = { summary = {
			findingCount = 2, uniqueVulnerabilityCount = 2, affectedWorkloadCount = 1,
			knownExploitedVulnerabilityCount = 1, knownExploitedWorkloadCount = 1,
			countsBySeverity = { critical = 1, high = 1, medium = 0, low = 0, unknown = 0 },
			coverage = { totalWorkloads = 3, scanned = 1, pending = 2, noSbom = 0, failed = 0, unresolvedImage = 0 },
		} },
		team = {
			vulnerabilityFindings = { summary = {
				findingCount = 2, uniqueVulnerabilityCount = 2, affectedWorkloadCount = 1,
				coverage = { totalWorkloads = 3, scanned = 1, pending = 2 },
			} },
			environment = { vulnerabilityFindings = { summary = {
				findingCount = 1, uniqueVulnerabilityCount = 1, affectedWorkloadCount = 1,
				knownExploitedVulnerabilityCount = 0, knownExploitedWorkloadCount = 0,
				countsBySeverity = { critical = 1, high = 0 },
				coverage = { totalWorkloads = 3, scanned = 1, pending = 2 },
			} } },
			empty = { summary = {
				findingCount = 0, uniqueVulnerabilityCount = 0, affectedWorkloadCount = 0,
				coverage = { totalWorkloads = 3, scanned = 1, pending = 2 },
			} },
		},
	} }
end)

Test.gql("Current findings use workload inventory and coexist with legacy advisories", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{
		vulnerability(identifier: "CVE-1") { id identifier references }
		cve(identifier: "CVE-2024-12345") { identifier }
		team(slug: "v0s-team") {
			vulnerabilityFindings(first: 1) {
				nodes { id vulnerabilityIdentifier severity fixedVersion workload { name } }
				edges { cursor }
				pageInfo { totalCount hasNextPage hasPreviousPage pageStart pageEnd endCursor }
				facets { fixableFindings knownExploitedFindings countsBySeverity { high critical } }
			}
			workloads(first: 1) { facets { vulnerabilityCoverage { totalWorkloads scanned pending } } }
			pending: workloads(filter: {vulnerabilityCoverageStatuses: [PENDING]}) {
				nodes { name __typename vulnerabilityCoverage { status imageDigest } }
			}
			environment(name: "dev") {
				vulnerabilityFindings(filter: {vulnerabilityIdentifiers: ["CVE-1", "missing"]}) {
					nodes { vulnerabilityIdentifier }
					pageInfo { totalCount }
				}
				workload(name: "app") {
					vulnerabilityFindings(filter: {onlyWithFix: true}) { pageInfo { totalCount } }
					vulnerabilityCoverage { status imageDigest }
				}
			}
		}
	}]]
	t.check { data = {
		vulnerability = { id = Save("advisory_id"), identifier = "CVE-1", references = { "https://example.com/advisories/CVE-1" } },
		cve = { identifier = "CVE-2024-12345" },
		team = {
			vulnerabilityFindings = {
				nodes = { { id = Save("finding_id"), vulnerabilityIdentifier = "CVE-1", severity = "HIGH", fixedVersion = Null, workload = { name = "app" } } },
				edges = { { cursor = Save("finding_cursor") } },
				pageInfo = { totalCount = 2, hasNextPage = true, hasPreviousPage = false, pageStart = 1, pageEnd = 1, endCursor = Save("end_cursor") },
				facets = { fixableFindings = 1, knownExploitedFindings = 1, countsBySeverity = { high = 1, critical = 1 } },
			},
			workloads = { facets = { vulnerabilityCoverage = { totalWorkloads = 3, scanned = 1, pending = 2 } } },
			pending = { nodes = {
				{ name = "fresh", __typename = "Job",         vulnerabilityCoverage = { status = "PENDING", imageDigest = Null } },
				{ name = "stale", __typename = "Application", vulnerabilityCoverage = { status = "PENDING", imageDigest = Null } },
			} },
			environment = {
				vulnerabilityFindings = { nodes = { { vulnerabilityIdentifier = "CVE-1" } }, pageInfo = { totalCount = 1 } },
				workload = { vulnerabilityFindings = { pageInfo = { totalCount = 1 } }, vulnerabilityCoverage = { status = "SCANNED", imageDigest = "sha256:app" } },
			},
		},
	} }
	assert(State.finding_cursor == State.end_cursor, "edge and page cursors differ")

	t.query(string.format([[{
		team(slug: "v0s-team") { vulnerabilityFindings(first: 1, after: "%s") {
			nodes { vulnerabilityIdentifier fixedVersion }
			pageInfo { totalCount hasNextPage hasPreviousPage pageStart pageEnd startCursor }
		} }
		finding: node(id: "%s") { ... on VulnerabilityFinding { vulnerabilityIdentifier vulnerability { identifier } } }
		advisory: node(id: "%s") { ... on Vulnerability { identifier } }
	}]], State.end_cursor, State.finding_id, State.advisory_id))
	t.check { data = {
		team = { vulnerabilityFindings = {
			nodes = { { vulnerabilityIdentifier = "CVE-2", fixedVersion = "2" } },
			pageInfo = { totalCount = 2, hasNextPage = false, hasPreviousPage = true, pageStart = 2, pageEnd = 2, startCursor = Save("second_finding_cursor") },
		} },
		finding = { vulnerabilityIdentifier = "CVE-1", vulnerability = { identifier = "CVE-1" } },
		advisory = { identifier = "CVE-1" },
	} }

	t.query(string.format([[{
		team(slug: "v0s-team") { vulnerabilityFindings(last: 20, before: "%s") {
			nodes { vulnerabilityIdentifier }
			edges { cursor }
			pageInfo { totalCount hasNextPage hasPreviousPage pageStart pageEnd startCursor endCursor }
			facets { fixableFindings knownExploitedFindings }
		} }
	}]], State.second_finding_cursor))
	t.check { data = { team = { vulnerabilityFindings = {
		nodes = { { vulnerabilityIdentifier = "CVE-1" } },
		edges = { { cursor = State.finding_cursor } },
		pageInfo = { totalCount = 2, hasNextPage = true, hasPreviousPage = false, pageStart = 1, pageEnd = 1,
			startCursor = State.finding_cursor, endCursor = State.finding_cursor },
		facets = { fixableFindings = 1, knownExploitedFindings = 1 },
	} } } }

	t.query(string.format([[{
		team(slug: "v0s-team") { vulnerabilityFindings(last: 1, before: "%s") {
			nodes { vulnerabilityIdentifier }
			pageInfo { totalCount hasNextPage hasPreviousPage startCursor endCursor pageStart pageEnd }
		} }
	}]], State.finding_cursor))
	t.check { data = { team = { vulnerabilityFindings = {
		nodes = {}, pageInfo = { totalCount = 2, hasNextPage = true, hasPreviousPage = false,
		startCursor = Null, endCursor = Null, pageStart = 0, pageEnd = 0 },
	} } } }
end)

Test.gql("Advisory lookup distinguishes ambiguous and missing identifiers", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{ vulnerability(identifier: "ambiguous") { identifier } }]]
	t.check { data = { vulnerability = Null }, errors = { { locations = NotNull(), message = "alias names several vulnerabilities", path = { "vulnerability" } } } }
	t.query [[{ vulnerability(identifier: "missing") { identifier } }]]
	t.check { data = { vulnerability = Null } }
end)

Test.gql("Finding ordering and package facets cover the full current selection", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{
		vulnerabilityFindings(first: 1, filter: {packageType: "NPM"}, orderBy: {field: SEVERITY, direction: DESC}) {
			nodes { vulnerabilityIdentifier severity }
			pageInfo { totalCount hasNextPage endCursor }
			facets { packageTypes { value count } countsBySeverity { high critical } }
		}
		ascending: vulnerabilityFindings(orderBy: {field: SEVERITY, direction: ASC}) {
			nodes { vulnerabilityIdentifier }
		}
		team(slug: "v0s-team") {
			vulnerabilityFindings(orderBy: {field: IDENTIFIER, direction: DESC}) {
				nodes { vulnerabilityIdentifier }
			}
			environment(name: "dev") {
				vulnerabilityFindings(orderBy: {field: IDENTIFIER, direction: ASC}) {
					nodes { vulnerabilityIdentifier }
				}
				workload(name: "app") {
					vulnerabilityFindings(first: 1, orderBy: {field: SEVERITY, direction: DESC}) {
						nodes { vulnerabilityIdentifier }
						facets { packageTypes { value count } }
					}
				}
				empty: workload(name: "fresh") {
					vulnerabilityFindings(orderBy: {field: SEVERITY, direction: DESC}) {
						nodes { vulnerabilityIdentifier }
						facets { packageTypes { value count } }
					}
				}
			}
		}
		missing: vulnerabilityFindings(filter: {packageType: "unknown"}) {
			nodes { vulnerabilityIdentifier }
			pageInfo { totalCount }
			facets { packageTypes { value count } }
		}
	}]]
	t.check { data = {
		vulnerabilityFindings = {
			nodes = { { vulnerabilityIdentifier = "CVE-2", severity = "CRITICAL" } },
			pageInfo = { totalCount = 2, hasNextPage = true, endCursor = Save("sorted_finding_cursor") },
			facets = { packageTypes = { { value = "npm", count = 2 } }, countsBySeverity = { high = 1, critical = 1 } },
		},
		ascending = { nodes = { { vulnerabilityIdentifier = "CVE-1" }, { vulnerabilityIdentifier = "CVE-2" } } },
		team = {
			vulnerabilityFindings = { nodes = { { vulnerabilityIdentifier = "CVE-2" }, { vulnerabilityIdentifier = "CVE-1" } } },
			environment = {
				vulnerabilityFindings = { nodes = { { vulnerabilityIdentifier = "CVE-1" }, { vulnerabilityIdentifier = "CVE-2" } } },
				workload = { vulnerabilityFindings = {
					nodes = { { vulnerabilityIdentifier = "CVE-2" } },
					facets = { packageTypes = { { value = "npm", count = 2 } } },
				} },
				empty = { vulnerabilityFindings = { nodes = {}, facets = { packageTypes = {} } } },
			},
		},
		missing = { nodes = {}, pageInfo = { totalCount = 0 }, facets = { packageTypes = {} } },
	} }

	t.query(string.format([[{
		vulnerabilityFindings(first: 1, after: "%s", filter: {packageType: "npm"}, orderBy: {field: SEVERITY, direction: DESC}) {
			nodes { vulnerabilityIdentifier }
			pageInfo { totalCount hasNextPage }
			facets { packageTypes { value count } }
		}
	}]], State.sorted_finding_cursor))
	t.check { data = { vulnerabilityFindings = {
		nodes = { { vulnerabilityIdentifier = "CVE-1" } },
		pageInfo = { totalCount = 2, hasNextPage = false },
		facets = { packageTypes = { { value = "npm", count = 2 } } },
	} } }
end)

Test.gql("Unique vulnerability groups have independent pages and full totals", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{
		team(slug: "v0s-team") {
			vulnerabilityFindings(first: 1, filter: {packageType: "NPM"}) {
				nodes { vulnerabilityIdentifier }
				vulnerabilities(first: 1) {
					nodes { vulnerabilityIdentifier severity findingCount affectedWorkloadCount vulnerability { identifier } }
					pageInfo { totalCount hasNextPage endCursor }
				}
				byWorkloads: vulnerabilities(orderBy: {field: AFFECTED_WORKLOAD_COUNT, direction: DESC}) {
					nodes { vulnerabilityIdentifier }
				}
			}
			fixed: vulnerabilityFindings(filter: {onlyWithFix: true}) {
				vulnerabilities {
					nodes { vulnerabilityIdentifier findingCount affectedWorkloadCount }
					pageInfo { totalCount }
				}
			}
		}
	}]]
	t.check { data = { team = {
		vulnerabilityFindings = {
			nodes = { { vulnerabilityIdentifier = "CVE-1" } },
			vulnerabilities = {
				nodes = { { vulnerabilityIdentifier = "CVE-2", severity = "CRITICAL", findingCount = 1, affectedWorkloadCount = 1,
					vulnerability = { identifier = "CVE-2" } } },
				pageInfo = { totalCount = 2, hasNextPage = true, endCursor = Save("vulnerability_overview_cursor") },
			},
			byWorkloads = { nodes = { { vulnerabilityIdentifier = "CVE-1" }, { vulnerabilityIdentifier = "CVE-2" } } },
		},
		fixed = { vulnerabilities = {
			nodes = { { vulnerabilityIdentifier = "CVE-2", findingCount = 1, affectedWorkloadCount = 1 } },
			pageInfo = { totalCount = 1 },
		} },
	} } }

	t.query(string.format([[{
		team(slug: "v0s-team") {
			vulnerabilityFindings(filter: {packageType: "npm"}, orderBy: {field: IDENTIFIER, direction: DESC}) {
				vulnerabilities(first: 1, after: "%s", orderBy: {field: SEVERITY, direction: DESC}) {
					nodes { vulnerabilityIdentifier }
					pageInfo { totalCount hasNextPage hasPreviousPage endCursor }
				}
			}
		}
	}]], State.vulnerability_overview_cursor))
	t.check { data = { team = { vulnerabilityFindings = { vulnerabilities = {
		nodes = { { vulnerabilityIdentifier = "CVE-1" } },
		pageInfo = { totalCount = 2, hasNextPage = false, hasPreviousPage = true, endCursor = Save("vulnerability_overview_last_cursor") },
	} } } } }

	t.query(string.format([[{
		team(slug: "v0s-team") {
			vulnerabilityFindings(filter: {packageType: "npm"}) {
				vulnerabilities(last: 20, before: "%s") {
					nodes { vulnerabilityIdentifier severity findingCount affectedWorkloadCount }
					pageInfo { totalCount hasNextPage hasPreviousPage pageStart pageEnd endCursor }
				}
			}
		}
	}]], State.vulnerability_overview_last_cursor))
	t.check { data = { team = { vulnerabilityFindings = { vulnerabilities = {
		nodes = { { vulnerabilityIdentifier = "CVE-2", severity = "CRITICAL", findingCount = 1, affectedWorkloadCount = 1 } },
		pageInfo = { totalCount = 2, hasNextPage = true, hasPreviousPage = false, pageStart = 1, pageEnd = 1,
			endCursor = State.vulnerability_overview_cursor },
	} } } } }

	t.query(string.format([[{
		team(slug: "v0s-team") {
			vulnerabilityFindings(filter: {packageType: "npm"}) {
				vulnerabilities(first: 1, after: "%s") {
					nodes { vulnerabilityIdentifier }
					pageInfo { totalCount hasNextPage }
				}
			}
		}
	}]], State.vulnerability_overview_last_cursor))
	t.check { data = { team = { vulnerabilityFindings = { vulnerabilities = {
		nodes = {}, pageInfo = { totalCount = 2, hasNextPage = false },
	} } } } }
end)

Test.gql("Existing workload pages retain pending workloads when enriched with vulnerability counts", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[{
		team(slug: "v0s-team") {
			workloads(first: 2) {
				nodes {
					name
					vulnerabilityOverview {
						findingCount uniqueVulnerabilityCount knownExploitedVulnerabilityCount
						countsBySeverity { high critical }
						coverage { status }
					}
				}
				pageInfo { totalCount hasNextPage endCursor }
			}
			environment(name: "dev") {
				workload(name: "app") {
					vulnerabilityOverview(filter: {onlyWithFix: true}) {
						findingCount uniqueVulnerabilityCount knownExploitedVulnerabilityCount
						countsBySeverity { high critical }
						coverage { status }
					}
				}
			}
		}
	}]]
	t.check { data = { team = {
		workloads = {
			nodes = {
				{ name = "app", vulnerabilityOverview = {
					findingCount = 2, uniqueVulnerabilityCount = 2, knownExploitedVulnerabilityCount = 1,
					countsBySeverity = { high = 1, critical = 1 }, coverage = { status = "SCANNED" },
				} },
				{ name = "fresh", vulnerabilityOverview = {
					findingCount = 0, uniqueVulnerabilityCount = 0, knownExploitedVulnerabilityCount = 0,
					countsBySeverity = { high = 0, critical = 0 }, coverage = { status = "PENDING" },
				} },
			},
			pageInfo = { totalCount = 3, hasNextPage = true, endCursor = Save("workload_overview_cursor") },
		},
		environment = { workload = { vulnerabilityOverview = {
			findingCount = 1, uniqueVulnerabilityCount = 1, knownExploitedVulnerabilityCount = 0,
			countsBySeverity = { high = 0, critical = 1 }, coverage = { status = "SCANNED" },
		} } },
	} } }

	t.query(string.format([[{
		team(slug: "v0s-team") {
			workloads(first: 2, after: "%s") {
				nodes {
					name
					vulnerabilityOverview {
						findingCount uniqueVulnerabilityCount countsBySeverity { high critical }
						coverage { status }
					}
				}
				pageInfo { totalCount hasNextPage }
			}
		}
	}]], State.workload_overview_cursor))
	t.check { data = { team = { workloads = {
		nodes = { { name = "stale", vulnerabilityOverview = {
			findingCount = 0, uniqueVulnerabilityCount = 0, countsBySeverity = { high = 0, critical = 0 },
			coverage = { status = "PENDING" },
		} } },
		pageInfo = { totalCount = 3, hasNextPage = false },
	} } } }
end)
