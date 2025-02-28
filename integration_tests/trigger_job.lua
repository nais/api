Helper.readK8sResources("k8s_resources/simple")

local user = User.new()
local nonMember = User.new()
local team = Team.new("slug-1", "purpose", "#slack_channel")
team:addMember(user)


Test.gql("job details", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		{
			team(slug: "slug-1") {
				environment(name: "dev") {
					job(name: "jobname-1") {
						runs {
							nodes {
								name
							}
							pageInfo {
								totalCount
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
						runs = {
							nodes = {
								{ name = "jobname-1-run1" },
							},
							pageInfo = {
								totalCount = 1,
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("as team member", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			triggerJob(
				input: {teamSlug: "slug-1", environmentName: "dev", name: "jobname-1", runName: "newRun"}
			) {
				jobRun {
					name
				}
			}
		}
	]]

	t.check {
		data = {
			triggerJob = {
				jobRun = {
					name = "newRun",
				},
			},
		},
	}
end)

Test.gql("as non-team member", function(t)
	t.addHeader("x-user-email", nonMember:email())

	t.query([[
		mutation {
			triggerJob(
				input: {teamSlug: "slug-1", environmentName: "dev", name: "jobname-1", runName: "newRun2"}
			) {
				jobRun {
					name
				}
			}
		}
	]])

	t.check {
		data = Null,
		errors = {
			{
				message = Contains("you need the \"jobs:update\""),
				path = { "triggerJob" },
			},
		},
	}
end)


Test.gql("job details after trigger", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		{
			team(slug: "slug-1") {
				activityLog {
					nodes {
						message
					}
				}
				environment(name: "dev") {
					job(name: "jobname-1") {
						runs {
							nodes {
								name
							}
							pageInfo {
								totalCount
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
				activityLog = {
					nodes = {
						{
							message = "Job triggered",
						},
					},
				},
				environment = {
					job = {
						runs = {
							nodes = {
								{ name = "jobname-1-run1" },
								{ name = "newRun" },
							},
							pageInfo = {
								totalCount = 2,
							},
						},
					},
				},
			},
		},
	}
end)
