Helper.readK8sResources("k8s_resources/status")
local user = User.new("name", "auth@user.com", "sdf")
local team = Team.new("slug-1", "purpose", "#slack_channel")

local function issueQuery(slug)
	return string.format([[
		query  {
  			team(slug: %s) {
     			slug
    issues {
      id
      resourceName
      resourceType
      environment
      team
      severity
      ... on AivenIssue {
        message
      }

    }
  }
}
]], slug or "")
end

Test.gql("team with no issues", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(issueQuery(team.slug))

	t.check {
		data = {
			team = {
				slug = team.slug,
				issues = {},
			},
		},
	}
end)
