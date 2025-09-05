Helper.readK8sResources("k8s_resources/issues")
local user = User.new("name", "auth@user.com", "sdf")
Team.new("myteam", "purpose", "#slack_channel")
local checker = IssueChecker.new()

Test.gql("Team with no issues ", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "myteam") {
				slug
				issues {
				  	nodes {
				      id
					}
				}
			}
		}
	]]

	t.check {
		data = {
			team = {
				slug = "myteam",
				issues = {
					nodes = {},
				},
			},
		},
	}
end)

Test.gql("Team with issues", function(t)
	checker:runChecks()
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "myteam") {
				slug
				issues(
					filter: {
						environments: ["dev-gcp"]
					},
					orderBy: {
						field: RESOURCE_NAME, direction: ASC}
				) {
					nodes {
						__typename
						environment
						severity
						message
						... on DeprecatedIngressIssue {
							ingresses
							application {
								name
							}
						}
						... on OpenSearchIssue {
							event
							openSearch {
								name
							}
						}
						... on SqlInstanceStateIssue {
							state
							sqlInstance {
								name
							}
						}
						... on SqlInstanceVersionIssue {
							sqlInstance {
								name
							}
						}
						... on DeprecatedRegistryIssue {
							workload {
								name
							}
						}
					}
				}
			}
		}
	]]

	t.check {
		data = {
			team = {
				slug = "myteam",
				issues = {
					nodes = {
						{
							__typename = "DeprecatedIngressIssue",
							environment = "dev-gcp",
							application = {
								name = "deprecated",
							},
							message = "Deprecated ingress detected",
							severity = "TODO",
							ingresses = { "https://error.dev-gcp.nais.io" },
						},
						{
							__typename = "SqlInstanceVersionIssue",
							environment = "dev-gcp",
							message = "The instance is running a deprecated version of PostgreSQL: POSTGRES_12",
							severity = "WARNING",
							sqlInstance = {
								name = "deprecated",
							},
						},
						{
							__typename = "DeprecatedRegistryIssue",
							environment = "dev-gcp",
							message = "Image 'deprecated.dev/nais/navikt/app-name:latest' is using a deprecated registry",
							severity = "WARNING",
							workload = {
								name = "deprecated",
							},
						},
						{
							__typename = "SqlInstanceStateIssue",
							environment = "dev-gcp",
							sqlInstance = {
								name = "maintenance",
							},
							severity = "CRITICAL",
							state = "MAINTENANCE",
							message = "The instance is down for maintenance.",
						},
						{
							__typename = "OpenSearchIssue",
							environment = "dev-gcp",
							message = "error message from aiven",
							event = "error message from aiven",
							openSearch = {
								name = "opensearch-myteam-name",
							},
							severity = "CRITICAL",
						},
						{
							__typename = "SqlInstanceStateIssue",
							environment = "dev-gcp",
							message = "The instance has been stopped.",
							severity = "CRITICAL",
							sqlInstance = {
								name = "stopped",
							},
							state = "STOPPED",
						},
					},
				},
			},
		},
	}
end)
