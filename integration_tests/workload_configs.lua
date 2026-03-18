Helper.readK8sResources("k8s_resources/configs")

local user = User.new("authenticated", "user@user.com", "ext")

Test.gql("Create team", function(t)
	t.addHeader("x-user-email", user:email())

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

Test.gql("Application with no configs", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		{
			team(slug: "myteam") {
				slug
				environment(name: "dev") {
					application(name: "app-with-no-configs") {
						configs {
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
						configs = {
							nodes = {},
						},
					},
				},
			},
		},
	}
end)

Test.gql("Application with config", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		{
			team(slug: "myteam") {
				slug
				dev: environment(name: "dev") {
					application(name: "app-with-configs-via-envfrom") {
						configs {
							nodes {
								name
							}
						}
					}
				}
				staging: environment(name: "staging") {
					application(name: "app-with-configs-via-filesfrom") {
						configs {
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
						configs = {
							nodes = {
								{
									name = "managed-config-in-dev",
								},
							},
						},
					},
				},
				staging = {
					application = {
						configs = {
							nodes = {
								{
									name = "managed-config-in-staging-used-with-filesfrom",
								},
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("Configs with workloads", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		{
			team(slug: "myteam") {
				slug
				configs {
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
				configs = {
					nodes = {
						{
							name = "managed-config-in-dev",
							workloads = {
								nodes = {
									{
										name = "app-with-configs-via-envfrom",
										environment = {
											name = "dev",
										},
									},
								},
							},
						},
						{
							name = "managed-config-in-staging",
							workloads = {
								nodes = {},
							},
						},
						{
							name = "managed-config-in-staging-used-with-filesfrom",
							workloads = {
								nodes = {
									{
										name = "app-with-configs-via-filesfrom",
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
