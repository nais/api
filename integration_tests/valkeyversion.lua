Helper.readK8sResources("./k8s_resources/simple")
Helper.setAivenVersion("valkey-slug-1-contests", "9.1.0")
local user = User.new()
local team = Team.new("slug-1", "purpose", "#channel")

Test.gql("Show version of Valkey instance", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
{
  team(slug: "%s") {
    valkeys {
      nodes {
        name
        version {
          actual
          desiredMajor
        }
      }
    }
  }
}]], team:slug()))

	t.check {
		data = {
			team = {
				valkeys = {
					nodes = {
						{
							name = "valkey-slug-1-contests",
							version = {
								actual = "9.1.0",
								desiredMajor = "V9_1",
							},
						},
					},
				},
			},
		},
	}
end)
