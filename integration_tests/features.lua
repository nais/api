Test.gql("list enabled features", function(t)
	t.query([[
		query {
			features {
				id
				unleash {
					id
					enabled
				}
				redis {
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
				redis = {
					id = "F_DuZv6up",
					enabled = true,
				},
				valkey = {
					id = "F_VuZv6up",
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
		name = "FeatureRedis",
		id = "F_DuZv6up",
	},
	{
		name = "FeatureValkey",
		id = "F_VuZv6up",
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
