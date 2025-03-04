local user = User.new("usersen", "usr@exam.com", "ei")

Test.gql("all environments", function(t)
	t.addHeader("x-user-email", user:email())

	t.query([[
		{
			environments {
				name
			}
		}
	]])

	t.check {
		data = {
			environments = {
				{
					name = "dev",
				},
				{
					name = "dev-fss",
				},
				{
					name = "dev-gcp",
				},
				{
					name = "staging",
				},
			},
		},
	}
end)

Test.gql("all environments with ordering", function(t)
	t.addHeader("x-user-email", user:email())

	t.query([[
		{
			environments(
				orderBy: {
					field: NAME,
					direction: ASC
				}
			) {
				name
			}
		}
	]])

	t.check {
		data = {
			environments = {
				{
					name = "dev",
				},
				{
					name = "dev-fss",
				},
				{
					name = "dev-gcp",
				},
				{
					name = "staging",
				},
			},
		},
	}

	t.query([[
		{
			environments(
				orderBy: {
					field: NAME,
					direction: DESC
				}
			) {
				name
			}
		}
	]])

	t.check {
		data = {
			environments = {
				{
					name = "staging",
				},
				{
					name = "dev-gcp",
				},
				{
					name = "dev-fss",
				},
				{
					name = "dev",
				},
			},
		},
	}
end)

Test.gql("single environment", function(t)
	t.addHeader("x-user-email", user:email())

	t.query([[
		{
			environment(name: "dev") {
				name
			}
		}
	]])

	t.check {
		data = {
			environment = {
				name = "dev",
			},
		},
	}
end)

Test.gql("single environment that does not exist", function(t)
	t.addHeader("x-user-email", user:email())

	t.query([[
		{
			environment(name: "some-non-existing-environment") {
				name
			}
		}
	]])

	t.check {
		data = Null,
		errors = {
			{
				message = "Environment \"some-non-existing-environment\" not found",
				path = { "environment" },
			},
		},
	}
end)
