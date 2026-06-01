Helper.readK8sResources("k8s_resources/workload_image")

local user = User.new()
local team = Team.new("slug-1", "purpose", "#channel")
team:addMember(user)

Test.gql("update application image via Image resource when spec.image is empty", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			updateApplication(
				input: {
					teamSlug: "slug-1"
					environmentName: "dev"
					name: "image-app"
					image: "ghcr.io/navikt/image-app:v2.0.0"
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
					name = "image-app",
				},
			},
		},
	}
end)

Test.k8s("application spec.image remains empty", function(t)
	t.check("nais.io/v1alpha1", "applications", "dev", "slug-1", "image-app", {
		apiVersion = "nais.io/v1alpha1",
		kind = "Application",
		metadata = Ignore(),
		spec = {
			ingresses = { "https://image-app.external.server.com" },
			replicas = Ignore(),
		},
		status = Ignore(),
	})
end)

Test.k8s("Image resource is updated", function(t)
	t.check("nais.io/v1", "images", "dev", "slug-1", "image-app", {
		apiVersion = "nais.io/v1",
		kind = "Image",
		metadata = Ignore(),
		spec = {
			image = "ghcr.io/navikt/image-app:v2.0.0",
		},
	})
end)

Test.gql("activity log uses effectiveImage as old value", function(t)
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
							resourceName = "image-app",
							data = {
								changedFields = {
									{ field = "spec.image", oldValue = "ghcr.io/navikt/image-app:v1.0.0", newValue = "ghcr.io/navikt/image-app:v2.0.0" },
								},
							},
						},
					},
				},
			},
		},
	}
end)
