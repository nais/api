Helper.readK8sResources("k8s_resources/status_workload_filter")

local user = User.new("name", "auth@user.com", "sdf")
Team.new("slug-1", "purpose", "#slack_channel")


local function statusQuery(slug, filter)
	return string.format([[
		query {
			team(slug: "%s") {
				workloads(filter: { states: [%s] }) {
					nodes {
						__typename
						name
						status {
							state
						}
					}
				}
			}
		}
	]], slug, filter)
end

Test.gql("filter NAIS", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(statusQuery("slug-1", "NAIS"))

	t.check {
		data = {
			team = {
				workloads = {
					nodes = {
						{
							__typename = "Application",
							name = "deprecated-ingress",
							status = {
								state = "NAIS",
							},
						},
						{
							__typename = "Application",
							name = "no-errors",
							status = {
								state = "NAIS",
							},
						},
						{
							__typename = "Job",
							name = "no-errors",
							status = {
								state = "NAIS",
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("filter NOT_NAIS", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(statusQuery("slug-1", "NOT_NAIS"))

	t.check {
		data = {
			team = {
				workloads = {
					nodes = {
						{
							__typename = "Application",
							name = "deprecated-registry",
							status = {
								state = "NOT_NAIS",
							},
						},
						{
							__typename = "Job",
							name = "deprecated-registry",
							status = {
								state = "NOT_NAIS",
							},
						},
						{
							__typename = "Application",
							name = "failed-synchronization",
							status = {
								state = "NOT_NAIS",
							},
						},
						{
							__typename = "Job",
							name = "failed-synchronization",
							status = {
								state = "NOT_NAIS",
							},
						},
						{
							__typename = "Application",
							name = "invalid-yaml",
							status = {
								state = "NOT_NAIS",
							},
						},
						{
							__typename = "Job",
							name = "invalid-yaml",
							status = {
								state = "NOT_NAIS",
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("filter FAILING", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(statusQuery("slug-1", "FAILING"))

	t.check {
		data = {
			team = {
				workloads = {
					nodes = {
						{
							__typename = "Job",
							name = "failing",
							status = {
								state = "FAILING",
							},
						},
						{
							__typename = "Application",
							name = "missing-instances",
							status = {
								state = "FAILING",
							},
						},
					},
				},
			},
		},
	}
end)
