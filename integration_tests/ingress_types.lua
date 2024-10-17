Helper.readK8sResources("k8s_resources/simple")

Test.gql("Ingress types", function(t)
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
									url = "https://another-app.external.server.com"
								}
							}
						},
						{
							name = "app-name",
							ingresses = {
								{
									type = "INTERNAL",
									url = "https://my-app.server.com"
								}
							}
						}
					},
					pageInfo = {
						totalCount = 2
					}
				}
			}
		}
	}
end)
