Helper.readK8sResources("k8s_resources/issues")
local user = User.new("name", "auth@user.com", "sdf")
Team.new("myteam", "purpose", "#slack_channel")


Test.gql("Team with no issues ", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "myteam") {
				slug
				issues { id }
			}
		}
	]]

	t.check {
		data = {
			team = {
				slug = "myteam",
				issues = {},
			},
		},
	}
end)

Test.gql("App with deprecated ingress", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "myteam") {
				issues {
					severity
					... on DeprecatedIngressIssue {
						ingresses
					}
				 }
			}
		}
	]]

	t.check {
		data = {
			team = {
				slug = "myteam",
				issues = {},
			},
		},
	}
end)
