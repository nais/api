Helper.readK8sResources("k8s_resources/state")
local user = User.new("name", "auth@user.com", "sdf")
Team.new("myteam", "purpose", "#slack_channel")

local function stateQuery(slug, env, name, resourceType)
	return string.format([[
		query {
			team(slug: "%s") {
				environment(name: "%s") {
					%s(name: "%s") {
			    	    state
					}
				}
			}
		}
	]], slug, env, resourceType, name)
end

Test.gql("Applications sorted by state", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "myteam") {
				applications(
					orderBy: {
						field: STATE,
						direction: DESC
					}
				) {
					nodes {
						name
						state
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
							name = "app-failing",
							state = "NOT_RUNNING",
						},
						{
							name = "app-no-instances",
							state = "NOT_RUNNING",
						},
						{
							name = "app-running",
							state = "RUNNING",
						},
					},
				},
			},
		},
	}
end)

Test.gql("Jobs sorted by state", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "myteam") {
				jobs(
					orderBy: {
						field: STATE,
						direction: DESC
					}
				) {
					nodes {
						name
						state
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
							name = "job-failed",
							state = "FAILED",
						},
						{
							name = "job-running",
							state = "RUNNING",
						},
						{
							name = "job-completed",
							state = "COMPLETED",
						},
						{
							name = "job-no-runs",
							state = "UNKNOWN",
						},
					},
				},
			},
		},
	}
end)

Test.gql("app with no instances should not be running", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(stateQuery("myteam", "dev", "app-no-instances", "application"))

	t.check {
		data = {
			team = {
				environment = {
					application = {
						state = "NOT_RUNNING",
					},
				},
			},
		},
	}
end)

Test.gql("failing app should not be running", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(stateQuery("myteam", "dev", "app-failing", "application"))

	t.check {
		data = {
			team = {
				environment = {
					application = {
						state = "NOT_RUNNING",
					},
				},
			},
		},
	}
end)

Test.gql("app with healthy instances is running", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(stateQuery("myteam", "dev", "app-running", "application"))

	t.check {
		data = {
			team = {
				environment = {
					application = {
						state = "RUNNING",
					},
				},
			},
		},
	}
end)

Test.gql("job with completed last run is completed", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(stateQuery("myteam", "dev", "job-completed", "job"))

	t.check {
		data = {
			team = {
				environment = {
					job = {
						state = "COMPLETED",
					},
				},
			},
		},
	}
end)

Test.gql("job with failed last run has failed", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(stateQuery("myteam", "dev", "job-failed", "job"))

	t.check {
		data = {
			team = {
				environment = {
					job = {
						state = "FAILED",
					},
				},
			},
		},
	}
end)

Test.gql("job with no runs has state unknown", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(stateQuery("myteam", "dev", "job-no-runs", "job"))

	t.check {
		data = {
			team = {
				environment = {
					job = {
						state = "UNKNOWN",
					},
				},
			},
		},
	}
end)

Test.gql("job with active runs is running", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(stateQuery("myteam", "dev", "job-running", "job"))

	t.check {
		data = {
			team = {
				environment = {
					job = {
						state = "RUNNING",
					},
				},
			},
		},
	}
end)

Test.gql("State is reported on OpenSearch", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(stateQuery("myteam", "dev", "opensearch-myteam-running", "openSearch"))

	t.check {
		data = {
			team = {
				environment = {
					openSearch = {
						state = "RUNNING",
					},
				},
			},
		},
	}
end)

Test.gql("State is reported on Valkey", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(stateQuery("myteam", "dev", "valkey-myteam-running", "valkey"))

	t.check {
		data = {
			team = {
				environment = {
					valkey = {
						state = "RUNNING",
					},
				},
			},
		},
	}
end)
