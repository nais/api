Helper.readK8sResources("k8s_resources/issues")
local user = User.new("name", "auth@user.com", "sdf")
Team.new("myteam", "purpose", "#slack_channel")
local checker = IssueChecker.new()

Test.gql("Workload with no issues ", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "myteam") {
				environment(name: "dev-gcp") {
			  		application(name: "no-issues") {
						issues {
							nodes {
								id
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
				environment = {
					application = {
						issues = {
							nodes = {},
						},
					},
				},
			},
		},
	}
end)

Test.gql("Application with issues", function(t)
	checker:runChecks()
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "myteam") {
				environment(name: "dev-gcp") {
			  		application(name: "deprecated-app") {
						issues {
							nodes {
								__typename
								severity
								message
								... on DeprecatedIngressIssue {
									ingresses
								}
							}
				  		}
					}
					job(name: "deprecated-job") {
						issues {
							nodes {
								__typename
								severity
								message
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
				environment = {
					application = {
						issues = {
							nodes = {
								{
									__typename = "DeprecatedIngressIssue",
									message = "Application is using deprecated ingresses: [https://foo.dev-gcp.nais.io https://bar.dev-gcp.nais.io]",
									severity = "TODO",
									ingresses = { "https://foo.dev-gcp.nais.io", "https://bar.dev-gcp.nais.io" },
								},
								{
									__typename = "DeprecatedRegistryIssue",
									message = "Image 'deprecated.dev/nais/navikt/app-name:latest' is using a deprecated registry",
									severity = "WARNING",
								},
							},
						},
					},
					job = {
						issues = {
							nodes = {
								{
									__typename = "DeprecatedRegistryIssue",
									message = "Image 'ghcr.io/navikt/app-name:latest' is using a deprecated registry",
									severity = "WARNING",
								},
							},
						},
					},
				},
			},
		},
	}
end)
