Helper.readK8sResources("k8s_resources/instance_groups")

local user = User.new("username", "user@example.com", "ext-id")
Team.new("myteam", "some purpose", "#channel")

Test.gql("Application with instance groups", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "myteam") {
				applications {
					nodes {
						name
						instanceGroups {
							name
							id
							image {
								name
								tag
							}
							readyInstances
							desiredInstances
							created
						}
					}
				}
			}
		}
	]]

	-- Only the current ReplicaSet (revision 2) should be included because
	-- the old one (revision 1) has 0 desired and 0 ready replicas.
	t.check {
		data = {
			team = {
				applications = {
					nodes = {
						{
							name = "myapp",
							instanceGroups = {
								{
									name = "myapp-abc123",
									id = NotNull(),
									image = {
										name = "ghcr.io/navikt/myapp",
										tag = "v1.2.3",
									},
									readyInstances = 2,
									desiredInstances = 2,
									created = "2024-01-15T10:00:00Z",
								},
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("Instance group with instances", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "myteam") {
				applications {
					nodes {
						name
						instanceGroups {
							name
							instances {
								name
								id
								restarts
								created
								status {
									state
									message
									ready
									lastExitReason
									lastExitCode
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
				applications = {
					nodes = {
						{
							name = "myapp",
							instanceGroups = {
								{
									name = "myapp-abc123",
									instances = {
										{
											name = "myapp-abc123-pod1",
											id = NotNull(),
											restarts = 0,
											created = "2024-01-15T10:01:00Z",
											status = {
												state = "RUNNING",
												message = "Running and ready.",
												ready = true,
												lastExitReason = Null,
												lastExitCode = Null,
											},
										},
										{
											name = "myapp-abc123-pod2",
											id = NotNull(),
											restarts = 2,
											created = "2024-01-15T10:02:00Z",
											status = {
												state = "RUNNING",
												message = "Running and ready.",
												ready = true,
												lastExitReason = Null,
												lastExitCode = Null,
											},
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

Test.gql("Instance group with environment variables", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "myteam") {
				applications {
					nodes {
						name
						instanceGroups {
							name
							environmentVariables {
								name
								value
								source {
									kind
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
				applications = {
					nodes = {
						{
							name = "myapp",
							instanceGroups = {
								{
									name = "myapp-abc123",
									environmentVariables = {
										-- Direct env var
										{
											name = "APP_ENV",
											value = "production",
											source = {
												kind = "SPEC",
												name = "myapp",
											},
										},
										-- ConfigMap key ref (value resolved from ConfigMap)
										{
											name = "DB_HOST",
											value = "postgres.myteam.svc.cluster.local",
											source = {
												kind = "CONFIG",
												name = "myapp-config",
											},
										},
										-- Secret key ref (value is null, requires elevation)
										{
											name = "API_KEY",
											value = Null,
											source = {
												kind = "SECRET",
												name = "myapp-secret",
											},
										},
										-- envFrom ConfigMap (individual keys resolved)
										{
											name = "FEATURE_FLAG_A",
											value = "true",
											source = {
												kind = "CONFIG",
												name = "myapp-envs",
											},
										},
										{
											name = "FEATURE_FLAG_B",
											value = "false",
											source = {
												kind = "CONFIG",
												name = "myapp-envs",
											},
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

Test.gql("Instance group with mounted files", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "myteam") {
				applications {
					nodes {
						name
						instanceGroups {
							name
						mountedFiles {
								path
								content
								encoding
								error
								source {
									kind
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
				applications = {
					nodes = {
						{
							name = "myapp",
							instanceGroups = {
								{
									name = "myapp-abc123",
									mountedFiles = {
										-- ConfigMap files (individual keys expanded, content included)
										{
											path = "/etc/config/db_host",
											content = "postgres.myteam.svc.cluster.local",
											encoding = "PLAIN_TEXT",
											error = Null,
											source = {
												kind = "CONFIG",
												name = "myapp-config",
											},
										},
										{
											path = "/etc/config/db_port",
											content = "5432",
											encoding = "PLAIN_TEXT",
											error = Null,
											source = {
												kind = "CONFIG",
												name = "myapp-config",
											},
										},
										{
											path = "/etc/config/log_level",
											content = "info",
											encoding = "PLAIN_TEXT",
											error = Null,
											source = {
												kind = "CONFIG",
												name = "myapp-config",
											},
										},
										-- Secret files (individual keys expanded, content is null)
										{
											path = "/etc/secret/api_key",
											content = Null,
											encoding = "PLAIN_TEXT",
											error = Null,
											source = {
												kind = "SECRET",
												name = "myapp-secret",
											},
										},
										{
											path = "/etc/secret/db_password",
											content = Null,
											encoding = "PLAIN_TEXT",
											error = Null,
											source = {
												kind = "SECRET",
												name = "myapp-secret",
											},
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
