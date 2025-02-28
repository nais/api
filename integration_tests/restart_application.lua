Helper.readK8sResources("k8s_resources/simple")

local user = User.new()
local nonMember = User.new()
local team = Team.new("slug-1", "purpose", "#channel")
team:addMember(user)

Test.gql("as team member", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			restartApplication(
				input: {teamSlug: "slug-1", environmentName: "dev", name: "another-app"}
			) {
				application {
					name
				}
			}
		}
	]]

	t.check {
		data = {
			restartApplication = {
				application = {
					name = "another-app",
				},
			},
		},
	}
end)

Test.gql("as non-team member", function(t)
	t.addHeader("x-user-email", nonMember:email())

	t.query([[
		mutation {
			restartApplication(
				input: {teamSlug: "slug-1", environmentName: "dev", name: "another-app"}
			) {
				application {
					name
				}
			}
		}
	]])

	t.check {
		data = Null,
		errors = {
			{
				message = Contains("you need the \"applications:update\""),
				path = { "restartApplication" },
			},
		},
	}
end)

Test.k8s("The resource has proper annotations", function(t)
	t.check("apps/v1", "deployments", "dev", "slug-1", "another-app", {
		apiVersion = "apps/v1",
		kind = "Deployment",
		metadata = Ignore(),
		spec = {
			replicas = Ignore(),
			selector = Ignore(),
			strategy = Ignore(),
			template = {
				spec = NotNull(),
				metadata = {
					annotations = {
						["kubectl.kubernetes.io/restartedAt"] = NotNull(),
						["prometheus.io/port"] = "8080",
						["prometheus.io/scrape"] = "true",
					},
					creationTimestamp = Ignore(),
					labels = Ignore(),
				},
			},
		},
		status = Ignore(),
	})
end)
