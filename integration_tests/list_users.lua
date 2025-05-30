-- Create 20 users
for i = 1, 20 do
	local email = string.format("email-%d@example.com", i)
	local name = string.format("name-%d", i)
	local externalID = string.format("external_id-%d", i)
	User.new(name, email, externalID)
end

local user = User.new("Authenticated User", "authenticated@example.com", "authenticated")

Test.gql("list users", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			users(first: 5) {
				nodes {
					name
					email
				}
				pageInfo {
					totalCount
					endCursor
					hasNextPage
					hasPreviousPage
				}
			}
		}
	]]

	t.check {
		data = {
			users = {
				nodes = {
					{
						email = "authenticated@example.com",
						name = "Authenticated User",
					},
					{
						email = "email-1@example.com",
						name = "name-1",
					},
					{
						email = "email-10@example.com",
						name = "name-10",
					},
					{
						email = "email-11@example.com",
						name = "name-11",
					},
					{
						email = "email-12@example.com",
						name = "name-12",
					},
				},
				pageInfo = {
					totalCount = 21,
					endCursor = Save("nextPageCursor"),
					hasNextPage = true,
					hasPreviousPage = false,
				},
			},
		},
	}
end)

Test.gql("list users with offset", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		query {
			users(first: 5 after:"%s") {
				nodes {
					name
					email
				}
				pageInfo {
					totalCount
					endCursor
					hasNextPage
					hasPreviousPage
				}
			}
		}
	]], State.nextPageCursor))

	t.check {
		data = {
			users = {
				nodes = {
					{
						email = "email-13@example.com",
						name = "name-13",
					},
					{
						email = "email-14@example.com",
						name = "name-14",
					},
					{
						email = "email-15@example.com",
						name = "name-15",
					},
					{
						email = "email-16@example.com",
						name = "name-16",
					},
					{
						email = "email-17@example.com",
						name = "name-17",
					},
				},
				pageInfo = {
					totalCount = 21,
					endCursor = Ignore(),
					hasNextPage = true,
					hasPreviousPage = true,
				},
			},
		},
	}
end)
