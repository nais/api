local user = User.new("user-1", "usr@ex.com", "ei")

Test.gql("list enabled features", function(t)
	t.addHeader("x-user-email", user:email())

	t.query([[
		query {
			features {
				id
				unleash {
					id
					enabled
				}
				valkey {
					id
					enabled
				}
				kafka {
					id
					enabled
				}
				openSearch {
					id
					enabled
				}
			}
		}
	]])

	t.check {
		data = {
			features = {
				id = "F_2GQkaB5uADMS9",
				unleash = {
					id = "F_5T7D7YtTPV",
					enabled = true,
				},
				valkey = {
					id = "F_21x71SHpk",
					enabled = true,
				},
				kafka = {
					id = "F_D7fH1tt",
					enabled = true,
				},
				openSearch = {
					id = "F_7G8VavnTRAcpbH",
					enabled = true,
				},
			},
		},
	}
end)

local nodeTests = {
	{
		name = "FeatureUnleash",
		id = "F_5T7D7YtTPV",
	},
	{
		name = "FeatureValkey",
		id = "F_21x71SHpk",
	},
	{
		name = "FeatureKafka",
		id = "F_D7fH1tt",
	},
	{
		name = "FeatureOpenSearch",
		id = "F_7G8VavnTRAcpbH",
	},
}

for _, nodeTest in ipairs(nodeTests) do
	Test.gql("get feature " .. nodeTest.name, function(t)
		t.addHeader("x-user-email", user:email())

		t.query(
			string.format([[
			query {
				node(id: "%s") {
					id
					... on %s {
						enabled
					}
				}
			}
			]], nodeTest.id, nodeTest.name)
		)

		t.check {
			data = {
				node = {
					id = nodeTest.id,
					enabled = true,
				},
			},
		}
	end)
end
