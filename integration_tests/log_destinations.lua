Helper.readK8sResources("k8s_resources/log_destinations")

local user = User.new("name", "email@email.com", "externalID")
Team.new("logteam", "purpose", "#slack_channel")

for _, environment in ipairs({ "dev", "staging" }) do
	Test.gql("Environment default log destinations in " .. environment, function(t)
		t.addHeader("x-user-email", user:email())

		t.query([[
			query {
				team(slug: "logteam") {
					environment(name: "]] .. environment .. [[") {
						defaultApplication: application(name: "app-environment-logs") {
							logDestinations { __typename ... on LogDestinationGeneric { name } }
						}
						enabledApplication: application(name: "app-enabled-logs") {
							logDestinations { __typename ... on LogDestinationGeneric { name } }
						}
						defaultJob: job(name: "job-environment-logs") {
							logDestinations { __typename ... on LogDestinationGeneric { name } }
						}
						enabledJob: job(name: "job-enabled-logs") {
							logDestinations { __typename ... on LogDestinationGeneric { name } }
						}
					}
				}
			}
		]])

		local destination = { __typename = "LogDestinationLoki" }
		if environment == "staging" then
			destination = { __typename = "LogDestinationSecureLogs" }
		end
		local expected = { logDestinations = { destination } }
		t.check {
			data = {
				team = {
					environment = {
						defaultApplication = expected,
						enabledApplication = expected,
						defaultJob = expected,
						enabledJob = expected,
					},
				},
			},
		}
	end)
end

Test.gql("Application with all log destination types (loki, secure_logs, generic)", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "logteam") {
				environment(name: "dev") {
					application(name: "app-all-logs") {
						name
						logDestinations {
							__typename
							id
							... on LogDestinationLoki {
								grafanaURL
							}
							... on LogDestinationSecureLogs {
								id
							}
							... on LogDestinationGeneric {
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
				environment = {
					application = {
						name = "app-all-logs",
						logDestinations = {
							{
								__typename = "LogDestinationLoki",
								id = NotNull(),
								grafanaURL = Contains("grafana"),
							},
							{
								__typename = "LogDestinationSecureLogs",
								id = NotNull(),
							},
							{
								__typename = "LogDestinationGeneric",
								id = NotNull(),
								name = "elastic",
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("Application with only generic log destinations", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "logteam") {
				environment(name: "dev") {
					application(name: "app-only-generic") {
						name
						logDestinations {
							__typename
							id
							... on LogDestinationGeneric {
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
				environment = {
					application = {
						name = "app-only-generic",
						logDestinations = {
							{
								__typename = "LogDestinationGeneric",
								id = NotNull(),
								name = "elastic",
							},
							{
								__typename = "LogDestinationGeneric",
								id = NotNull(),
								name = "custom-sink",
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("Application with no log destinations defaults to loki", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "logteam") {
				environment(name: "dev") {
					application(name: "app-default-logs") {
						name
						logDestinations {
							__typename
							id
							... on LogDestinationLoki {
								grafanaURL
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
						name = "app-default-logs",
						logDestinations = {
							{
								__typename = "LogDestinationLoki",
								id = NotNull(),
								grafanaURL = Contains("grafana"),
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("Job with multiple log destinations returns correct types", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "logteam") {
				environment(name: "dev") {
					job(name: "job-multi-logs") {
						name
						logDestinations {
							__typename
							id
							... on LogDestinationLoki {
								grafanaURL
							}
							... on LogDestinationGeneric {
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
				environment = {
					job = {
						name = "job-multi-logs",
						logDestinations = {
							{
								__typename = "LogDestinationLoki",
								id = NotNull(),
								grafanaURL = Contains("grafana"),
							},
							{
								__typename = "LogDestinationGeneric",
								id = NotNull(),
								name = "elastic",
							},
						},
					},
				},
			},
		},
	}
end)
