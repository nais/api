Helper.readK8sResources("k8s_resources/issues")
local user = User.new("name", "auth@user.com", "sdf")
local team = Team.new("myteam", "purpose", "#slack_channel")

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

Test.gql("Team with issues", function(t)
	team:runChecks()
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "myteam") {
				slug
					issues {
						__typename
						environment
						resourceName
						resourceType
						severity
						... on DeprecatedIngressIssue {
							ingresses
						}
						... on AivenIssue {
							message
						}
						... on SQLInstanceIssue {
							state
							message
						 }
					}
				}
			}
		]]

	t.check {
		data = {
			team = {
				slug = "myteam",
				issues = {
					{
						__typename = "AivenIssue",
						environment = "dev",
						resourceName = "opensearch-myteam-name",
						resourceType = "opensearch",
						severity = "CRITICAL",
						message = "error message from aiven",
					},
					{
						__typename = "SQLInstanceIssue",
						environment = "dev-gcp",
						resourceName = "maintenance",
						resourceType = "sqlinstance",
						severity = "CRITICAL",
						state = "MAINTENANCE",
						message = "The instance is down for maintenance.",
					},
					{
						__typename = "SQLInstanceIssue",
						environment = "dev-gcp",
						resourceName = "stopped",
						resourceType = "sqlinstance",
						severity = "CRITICAL",
						state = "STOPPED",
						message = "The instance has been stopped",
					},
					{
						__typename = "DeprecatedIngressIssue",
						environment = "dev-gcp",
						resourceName = "deprecated-ingress",
						resourceType = "application",
						severity = "TODO",
						ingresses = { "https://error.dev-gcp.nais.io" },
					},
				},
			},
		},
	}
end)
