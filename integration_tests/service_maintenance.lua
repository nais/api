Helper.readK8sResources("./k8s_resources/simple")
local user = User.new()
local team = Team.new("slug-1", "purpose", "#channel")

Test.gql("Show maintenance updates for Valkey", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
{
  team(slug: "%s") {
    valkeyInstances {
      edges {
        node {
       	name
        project
        maintenance {
            updates {
              description
              documentationLink
              startAt
              deadline
            }
          }
        }
      }
    }
  }
}	]], team:slug()))

	t.check {
		data = {
			team = {
				valkeyInstances = {
					edges = {
						{
							node = {
								name = "valkey-slug-1-contests",
								project = "nav-dev",
								maintenance = {
									updates = {
										{
											description = "This is the impact (Nais API call it description)",
											documentationLink = "https://nais.io",
											startAt = Null,
											deadline = Null,
										},
										{
											description = "This is the impact (Nais API call it description)",
											documentationLink = Null,
											startAt = "1987-07-09T00:00:00Z",
											deadline = "1987-07-10T00:00:00Z",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
end)
