Helper.readK8sResources("k8s_resources/application_image_digest")

local user = User.new()
local team = Team.new("myteam", "some purpose", "#channel")
team:addMember(user)

Test.gql("application image digest is populated from pod status", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "myteam") {
				applications {
					nodes {
						name
						image {
							name
							tag
							digest
						}
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
							name = "myapp",
							image = {
								name = "ghcr.io/navikt/myapp",
								tag = "v1.2.3",
								digest = "sha256:abc123def456abc123def456abc123def456abc123def456abc123def456abc1",
							},
						},
					},
				},
			},
		},
	}
end)
