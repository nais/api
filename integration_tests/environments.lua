Helper.readK8sResources("k8s_resources/simple")

local user = User.new("usersen", "usr@exam.com", "ei")
Team.new("slug-1", "purpose", "#slack_channel")
Team.new("slug-2", "purpose", "#slack_channel")

Test.gql("all environments", function(t)
	t.addHeader("x-user-email", user:email())

	t.query([[
		{
			environments {
				nodes {
					name
				}
			}
		}
	]])

	t.check {
		data = {
			environments = {
				nodes = {
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
				nodes {
					name
				}
			}
		}
	]])

	t.check {
		data = {
			environments = {
				nodes = {
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
				nodes {
					name
				}
			}
		}
	]])

	t.check {
		data = {
			environments = {
				nodes = {
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

Test.gql("workloads in environment", function(t)
	t.addHeader("x-user-email", user:email())

	t.query([[
		{
			environment(name: "dev") {
				workloads {
					pageInfo {
						totalCount
					}
					nodes {
						__typename
						name
						team {
							slug
						}
					}
				}
			}
		}
	]])

	t.check {
		data = {
			environment = {
				workloads = {
					pageInfo = {
						totalCount = 5,
					},
					nodes = {
						{
							__typename = "Application",
							name = "another-app",
							team = {
								slug = "slug-1",
							},
						},
						{
							__typename = "Application",
							name = "app-name",
							team = {
								slug = "slug-1",
							},
						},
						{
							__typename = "Application",
							name = "app-name",
							team = {
								slug = "slug-2",
							},
						},
						{
							__typename = "Job",
							name = "jobname-1",
							team = {
								slug = "slug-1",
							},
						},
						{
							__typename = "Job",
							name = "jobname-2",
							team = {
								slug = "slug-1",
							},
						},
					},
				},
			},
		},
	}
end)
