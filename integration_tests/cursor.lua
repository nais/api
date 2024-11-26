Test.gql("forward pagination", function(t)
	local fetchTeams = function(after)
		t.query(string.format([[
			query {
				teams(first:5 after:"%s") {
					pageInfo {
						hasNextPage
						endCursor
					}
					edges {
						node {
							slug
						}
						cursor
					}
				}
			}
		]], after))
	end

	fetchTeams("")
	t.check {
		data = {
			teams = {
				pageInfo = {
					hasNextPage = true,
					endCursor = Save("endCursor"),
				},
				edges = {
					{
						node = {
							slug = "slug-1",
						},
						cursor = Ignore(),
					},
					{
						node = {
							slug = "slug-10",
						},
						cursor = Ignore(),
					},
					{
						node = {
							slug = "slug-11",
						},
						cursor = Ignore(),
					},
					{
						node = {
							slug = "slug-12",
						},
						cursor = Ignore(),
					},
					{
						node = {
							slug = "slug-13",
						},
						cursor = Save("lastNodeInPageCursor"),
					},
				},
			},
		},
	}

	assert(State.lastNodeInPageCursor == State.endCursor, "lastNodeInPageCursor is not equal to endCursor")

	fetchTeams(State.endCursor)
	t.check {
		data = {
			teams = {
				pageInfo = {
					hasNextPage = true,
					endCursor = Save("endCursor"),
				},
				edges = {
					{
						node = {
							slug = "slug-14",
						},
						cursor = Ignore(),
					},
					{
						node = {
							slug = "slug-15",
						},
						cursor = Ignore(),
					},
					{
						node = {
							slug = "slug-16",
						},
						cursor = Ignore(),
					},
					{
						node = {
							slug = "slug-17",
						},
						cursor = Ignore(),
					},
					{
						node = {
							slug = "slug-18",
						},
						cursor = Save("lastNodeInPageCursor"),
					},
				},
			},
		},
	}

	assert(State.lastNodeInPageCursor == State.endCursor, "lastNodeInPageCursor is not equal to endCursor")

	fetchTeams(State.endCursor)
	t.check {
		data = {
			teams = {
				pageInfo = {
					hasNextPage = true,
					endCursor = Save("endCursor"),
				},
				edges = {
					{
						node = {
							slug = "slug-19",
						},
						cursor = Ignore(),
					},
					{
						node = {
							slug = "slug-2",
						},
						cursor = Ignore(),
					},
					{
						node = {
							slug = "slug-20",
						},
						cursor = Ignore(),
					},
					{
						node = {
							slug = "slug-3",
						},
						cursor = Ignore(),
					},
					{
						node = {
							slug = "slug-4",
						},
						cursor = Save("lastNodeInPageCursor"),
					},

				},
			},
		},
	}

	assert(State.lastNodeInPageCursor == State.endCursor, "lastNodeInPageCursor is not equal to endCursor")

	fetchTeams(State.endCursor)
	t.check {
		data = {
			teams = {
				pageInfo = {
					hasNextPage = false,
					endCursor = Save("endCursor"),
				},
				edges = {
					{
						node = {
							slug = "slug-5",
						},
						cursor = Ignore(),
					},
					{
						node = {
							slug = "slug-6",
						},
						cursor = Ignore(),
					},
					{
						node = {
							slug = "slug-7",
						},
						cursor = Ignore(),
					},
					{
						node = {
							slug = "slug-8",
						},
						cursor = Ignore(),
					},
					{
						node = {
							slug = "slug-9",
						},
						cursor = Save("lastNodeInPageCursor"),
					},
				},
			},
		},
	}

	assert(State.lastNodeInPageCursor == State.endCursor, "lastNodeInPageCursor is not equal to endCursor")
end)
