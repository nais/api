Helper.readK8sResources("k8s_resources/simple")

local user = User.new("user-1", "usr@ex.com", "ei")
Team.new("slug-1", "team-name", "#team")


Test.gql("Ingress types", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "slug-1") {
				applications {
					nodes {
						name
						ingresses {
							url
							type
						}
					}
					pageInfo {
						totalCount
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
							name = "another-app",
							ingresses = {
								{
									type = "EXTERNAL",
									url = "https://another-app.external.server.com",
								},
							},
						},
						{
							name = "app-name",
							ingresses = {
								{
									type = "INTERNAL",
									url = "https://my-app.server.com",
								},
							},
						},
					},
					pageInfo = {
						totalCount = 2,
					},
				},
			},
		},
	}
end)
