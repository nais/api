Config.Unauthenticated = true

Test.gql("list users with unauthenticated request", function(t)
	t.query [[
		query {
			users(first: 5) {
				nodes {
					name
				}
			}
		}
	]]

	t.check {
		errors = {
			{
				message = "Unauthorized",
			},
		},
	}
end)
