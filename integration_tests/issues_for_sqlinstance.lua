Helper.readK8sResources("k8s_resources/issues")
local user = User.new("name", "auth@user.com", "sdf")
Team.new("myteam", "purpose", "#slack_channel")
local checker = IssueChecker.new()

Test.gql("SqlInstance with issues", function(t)
	checker:runChecks()
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "myteam") {
				environment(name: "dev-gcp") {
			  		sqlInstance(name: "stopped") {
						issues {
							nodes {
								__typename
								severity
								message
								... on SqlInstanceStateIssue {
									state
								}
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
				environment = {
					sqlInstance = {
						issues = {
							nodes = {
								{
									__typename = "SqlInstanceStateIssue",
									message = "The instance has been stopped.",
									severity = "CRITICAL",
									state = "STOPPED",
								},
							},
						},
					},
				},
			},
		},
	}
end)
