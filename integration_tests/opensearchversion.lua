Helper.readK8sResources("./k8s_resources/simple")
local user = User.new()
local team = Team.new("slug-1", "purpose", "#channel")

Test.gql("Show version of OpenSearch instance", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
{
  team(slug: "%s") {
    openSearches {
      nodes {
        name
        version
      }
    }
  }
}]], team:slug()))

	t.check {
		data = {
			team = {
				openSearches = {
					nodes = {
						{
							name = "opensearch-slug-1-opensearch",
							version = "2.17.2",
						},
					},
				},
			},
		},
	}
end)
