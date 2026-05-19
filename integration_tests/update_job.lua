Helper.readK8sResources("k8s_resources/simple")

local user = User.new()
local nonMember = User.new()
local team = Team.new("slug-1", "purpose", "#channel")
team:addMember(user)

Test.gql("update job env as team member", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			updateJob(
				input: {
					teamSlug: "slug-1"
					environmentName: "dev"
					name: "jobname-1"
					env: [
						{ name: "MY_VAR", value: "hello" }
						{ name: "OTHER", value: "world" }
					]
				}
			) {
				job {
					name
				}
			}
		}
	]]

	t.check {
		data = {
			updateJob = {
				job = {
					name = "jobname-1",
				},
			},
		},
	}
end)

Test.gql("update job no-op returns job without activity log", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			updateJob(
				input: {
					teamSlug: "slug-1"
					environmentName: "dev"
					name: "jobname-1"
					env: [
						{ name: "MY_VAR", value: "hello" }
						{ name: "OTHER", value: "world" }
					]
				}
			) {
				job {
					name
				}
			}
		}
	]]

	t.check {
		data = {
			updateJob = {
				job = {
					name = "jobname-1",
				},
			},
		},
	}
end)

Test.gql("update job as non-member", function(t)
	t.addHeader("x-user-email", nonMember:email())

	t.query [[
		mutation {
			updateJob(
				input: {
					teamSlug: "slug-1"
					environmentName: "dev"
					name: "jobname-1"
					env: [{ name: "MY_VAR", value: "hello" }]
				}
			) {
				job {
					name
				}
			}
		}
	]]

	t.check {
		data = Null,
		errors = {
			{
				locations = NotNull(),
				message = Contains("you need the"),
				path = { "updateJob" },
			},
		},
	}
end)

Test.k8s("job has updated env", function(t)
	t.check("nais.io/v1", "naisjobs", "dev", "slug-1", "jobname-1", {
		apiVersion = "nais.io/v1",
		kind = "Naisjob",
		metadata = Ignore(),
		spec = {
			image = "europe-north1-docker.pkg.dev/nais/navikt/app-name:latest",
			schedule = "0 0 * * *",
			env = {
				{ name = "MY_VAR", value = "hello" },
				{ name = "OTHER", value = "world" },
			},
		},
		status = Ignore(),
	})
end)

Test.gql("activity log contains job update entry", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		{
			team(slug: "%s") {
				activityLog(first: 10, filter: { activityTypes: [JOB_UPDATED] }) {
					nodes {
						__typename
						message
						actor
						resourceName
						... on JobUpdatedActivityLogEntry {
							data {
								changedFields {
									field
									oldValue
									newValue
								}
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
				activityLog = {
					nodes = {
						{
							__typename = "JobUpdatedActivityLogEntry",
							message = "Job jobname-1 updated",
							actor = user:email(),
							resourceName = "jobname-1",
							data = {
								changedFields = {
									{ field = "spec.env.MY_VAR", oldValue = Null, newValue = "hello" },
									{ field = "spec.env.OTHER", oldValue = Null, newValue = "world" },
								},
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("update job remove env variable", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			updateJob(
				input: {
					teamSlug: "slug-1"
					environmentName: "dev"
					name: "jobname-1"
					env: [
						{ name: "MY_VAR", value: null }
					]
				}
			) {
				job {
					name
				}
			}
		}
	]]

	t.check {
		data = {
			updateJob = {
				job = {
					name = "jobname-1",
				},
			},
		},
	}
end)

Test.k8s("job env after removal", function(t)
	t.check("nais.io/v1", "naisjobs", "dev", "slug-1", "jobname-1", {
		apiVersion = "nais.io/v1",
		kind = "Naisjob",
		metadata = Ignore(),
		spec = {
			image = "europe-north1-docker.pkg.dev/nais/navikt/app-name:latest",
			schedule = "0 0 * * *",
			env = {
				{ name = "OTHER", value = "world" },
			},
		},
		status = Ignore(),
	})
end)

Test.gql("activity log contains env removal entry", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		{
			team(slug: "%s") {
				activityLog(first: 1, filter: { activityTypes: [JOB_UPDATED] }) {
					nodes {
						__typename
						resourceName
						... on JobUpdatedActivityLogEntry {
							data {
								changedFields {
									field
									oldValue
									newValue
								}
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
				activityLog = {
					nodes = {
						{
							__typename = "JobUpdatedActivityLogEntry",
							resourceName = "jobname-1",
							data = {
								changedFields = {
									{ field = "spec.env.MY_VAR", oldValue = "hello", newValue = Null },
								},
							},
						},
					},
				},
			},
		},
	}
end)
