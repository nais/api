Helper.readK8sResources("k8s_resources/simple")

local user = User.new()
local nonMember = User.new()
local team = Team.new("slug-1", "purpose", "#channel")
team:addMember(user)

Test.gql("update application env and replicas as team member", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			updateApplication(
				input: {
					teamSlug: "slug-1"
					environmentName: "dev"
					name: "another-app"
					env: [
						{ name: "FOO", value: "bar" }
						{ name: "BAZ", value: "qux" }
					]
					replicas: { min: 2, max: 5 }
				}
			) {
				application {
					name
				}
			}
		}
	]]

	t.check {
		data = {
			updateApplication = {
				application = {
					name = "another-app",
				},
			},
		},
	}
end)

Test.k8s("application has updated env and replicas", function(t)
	t.check("nais.io/v1alpha1", "applications", "dev", "slug-1", "another-app", {
		apiVersion = "nais.io/v1alpha1",
		kind = "Application",
		metadata = Ignore(),
		spec = {
			image = "navikt/app-name:latest",
			ingresses = { "https://another-app.external.server.com" },
			replicas = Ignore(),
			env = {
				{ name = "BAZ", value = "qux" },
				{ name = "FOO", value = "bar" },
			},
		},
		status = Ignore(),
	})
end)

Test.gql("update application no-op returns application without new activity log", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			updateApplication(
				input: {
					teamSlug: "slug-1"
					environmentName: "dev"
					name: "another-app"
					env: [
						{ name: "FOO", value: "bar" }
						{ name: "BAZ", value: "qux" }
					]
					replicas: { min: 2, max: 5 }
				}
			) {
				application {
					name
				}
			}
		}
	]]

	t.check {
		data = {
			updateApplication = {
				application = {
					name = "another-app",
				},
			},
		},
	}
end)

Test.gql("update application as non-member", function(t)
	t.addHeader("x-user-email", nonMember:email())

	t.query [[
		mutation {
			updateApplication(
				input: {
					teamSlug: "slug-1"
					environmentName: "dev"
					name: "another-app"
					env: [{ name: "FOO", value: "bar" }]
				}
			) {
				application {
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
				path = { "updateApplication" },
			},
		},
	}
end)

Test.gql("activity log contains update entry", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		{
			team(slug: "%s") {
				activityLog(first: 10, filter: { activityTypes: [APPLICATION_UPDATED] }) {
					nodes {
						__typename
						message
						actor
						resourceName
						... on ApplicationUpdatedActivityLogEntry {
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
							__typename = "ApplicationUpdatedActivityLogEntry",
							message = "Application another-app updated",
							actor = user:email(),
							resourceName = "another-app",
							data = {
								changedFields = {
									{ field = "spec.env.BAZ",      oldValue = Null, newValue = "qux" },
									{ field = "spec.env.FOO",      oldValue = Null, newValue = "bar" },
									{ field = "spec.replicas.min", oldValue = "1",  newValue = "2" },
									{ field = "spec.replicas.max", oldValue = "1",  newValue = "5" },
								},
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("update application remove env variable", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			updateApplication(
				input: {
					teamSlug: "slug-1"
					environmentName: "dev"
					name: "another-app"
					env: [
						{ name: "FOO", value: null }
					]
				}
			) {
				application {
					name
				}
			}
		}
	]]

	t.check {
		data = {
			updateApplication = {
				application = {
					name = "another-app",
				},
			},
		},
	}
end)

Test.k8s("application env after removal", function(t)
	t.check("nais.io/v1alpha1", "applications", "dev", "slug-1", "another-app", {
		apiVersion = "nais.io/v1alpha1",
		kind = "Application",
		metadata = Ignore(),
		spec = {
			image = "navikt/app-name:latest",
			ingresses = { "https://another-app.external.server.com" },
			replicas = Ignore(),
			env = {
				{ name = "BAZ", value = "qux" },
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
				activityLog(first: 1, filter: { activityTypes: [APPLICATION_UPDATED] }) {
					nodes {
						__typename
						resourceName
						... on ApplicationUpdatedActivityLogEntry {
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
							__typename = "ApplicationUpdatedActivityLogEntry",
							resourceName = "another-app",
							data = {
								changedFields = {
									{ field = "spec.env.FOO", oldValue = "bar", newValue = Null },
								},
							},
						},
					},
				},
			},
		},
	}
end)
