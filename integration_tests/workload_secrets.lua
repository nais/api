Helper.readK8sResources("k8s_resources/secrets")

Test.gql("Create team", function(t)
	t.query [[
		mutation {
			createTeam(
				input: {
					slug: "myteam"
					purpose: "some purpose"
					slackChannel: "#channel"
				}
			) {
				team {
					slug
				}
			}
		}
	]]

	t.check {
		data = {
			createTeam = {
				team = {
					slug = "myteam",
				},
			},
		},
	}
end)

Test.gql("Application with no secrets", function(t)
	t.query [[
		{
			team(slug: "myteam") {
				slug
				environment(name: "dev") {
					application(name: "app-with-no-secrets") {
						secrets {
							nodes {
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
				environment = {
					application = {
						secrets = {
							nodes = {},
						},
					},
				},
			},
		},
	}
end)

Test.gql("Application with secret", function(t)
	t.query [[
		{
			team(slug: "myteam") {
				slug
				dev: environment(name: "dev") {
					application(name: "app-with-secrets-via-envfrom") {
						secrets {
							nodes {
								name
							}
						}
					}
				}
				staging: environment(name: "staging") {
					application(name: "app-with-secrets-via-filesfrom") {
						secrets {
							nodes {
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
				dev = {
					application = {
						secrets = {
							nodes = {
								{
									name = "managed-secret-in-dev",
								},
							},
						},
					},
				},
				staging = {
					application = {
						secrets = {
							nodes = {
								{
									name = "managed-secret-in-staging-used-with-filesfrom",
								},
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("Secrets with workloads", function(t)
	t.query [[
		{
			team(slug: "myteam") {
				slug
				secrets {
					nodes {
						name
						workloads {
							nodes {
								name
								environment {
									name
								}
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
				secrets = {
					nodes = {
						{
							name = "managed-secret-in-dev",
							workloads = {
								nodes = {
									{
										name = "app-with-secrets-via-envfrom",
										environment = {
											name = "dev",
										},
									},
								},
							},
						},
						{
							name = "managed-secret-in-staging",
							workloads = {
								nodes = {},
							},
						},
						{
							name = "managed-secret-in-staging-used-with-filesfrom",
							workloads = {
								nodes = {
									{
										name = "app-with-secrets-via-filesfrom",
										environment = {
											name = "staging",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
end)
