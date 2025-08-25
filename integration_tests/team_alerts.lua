Helper.readK8sResources("k8s_resources/simple")

local user = User.new("name", "email@email.com", "externalID")
Team.new("slug-1", "purpose", "#slack_channel")

Test.gql("Team with alerts", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "slug-1") {
				alerts(first: 1) {
					pageInfo {
						totalCount
					}
					nodes {
						name
						team {
							slug
						}
						teamEnvironment {
							environment {
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
				alerts = {
					nodes = {
						{
							name = "HTTPErrorRateTooHigh",
							team = {
								slug = "slug-1",
							},
							teamEnvironment = {
								environment = {
									name = "dev",
								},
							},
						},
					},
					pageInfo = {
						totalCount = 12,
					},
				},
			},
		},
	}
end)
