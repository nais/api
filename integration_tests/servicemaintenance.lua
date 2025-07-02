Helper.readK8sResources("./k8s_resources/simple")
local user = User.new()
local team = Team.new("slug-1", "purpose", "#channel")

Test.gql("Show maintenance window and updates for Valkey", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
{
  team(slug: "%s") {
    valkeyInstances {
      nodes {
        name
        maintenance {
          window {
            dayOfWeek
            timeOfDay
          }
          updates {
            nodes {
              title
              deadline
              description
              startAt
            }
          }
        }
      }
    }
  }
}]], team:slug()))

	t.check {
		data = {
			team = {
				valkeyInstances = {
					nodes = {
						{
							name = "valkey-slug-1-contests",
							maintenance = {
								window = {
									dayOfWeek = "sunday",
									timeOfDay = "12:34:56",
								},
								updates = {
									nodes = {
										{
											title = "This is a description (Nais API call it title)",
											description = "This is the impact (Nais API call it description)",
											startAt = Null,
											deadline = Null,
										},
										{
											title = "This is a description (Nais API call it title)",
											description = "This is the impact (Nais API call it description)",
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

Test.gql("Show maintenance window and updates for OpenSearch", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
{
  team(slug: "%s") {
    openSearchInstances {
      nodes {
        name
        maintenance {
          window {
            dayOfWeek
            timeOfDay
          }
          updates {
            nodes {
              title
              deadline
              description
              startAt
            }
          }
        }
      }
    }
  }
}]], team:slug()))

	t.check {
		data = {
			team = {
				openSearchInstances = {
					nodes = {
						{
							name = "opensearch-slug-1-opensearch",
							maintenance = {
								window = {
									dayOfWeek = "sunday",
									timeOfDay = "12:34:56",
								},
								updates = {
									nodes = {
										{
											title = "This is a description (Nais API call it title)",
											description = "This is the impact (Nais API call it description)",
											startAt = Null,
											deadline = Null,
										},
										{
											title = "This is a description (Nais API call it title)",
											description = "This is the impact (Nais API call it description)",
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
