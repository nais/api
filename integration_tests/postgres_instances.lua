local user = User.new("user", "user@usersen.com")

local mainTeam = Team.new("someteamname", "purpose", "#slack_channel")
mainTeam:addMember(user)

Helper.readK8sResources("k8s_resources/postgres_instances")

Test.gql("List postgres instances for team", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		{
		  team(slug: "someteamname") {
		    postgresInstances(orderBy: {field: NAME, direction: ASC}) {
		      nodes {
		        name
		        majorVersion
		        resources {
		          cpu
		          memory
		          diskSize
		        }
		        audit {
		          enabled
		        }
		      }
		    }
		  }
		}
	]]

	t.check {
		data = {
			team = {
				postgresInstances = {
					nodes = {
						{
							name = "another-db",
							majorVersion = "16",
							resources = {
								cpu = "200m",
								memory = "4G",
								diskSize = "10Gi",
							},
							audit = {
								enabled = false,
							},
						},
						{
							name = "foobar",
							majorVersion = "17",
							resources = {
								cpu = "100m",
								memory = "2G",
								diskSize = "2Gi",
							},
							audit = {
								enabled = false,
							},
						},
						{
							name = "with-audit",
							majorVersion = "16",
							resources = {
								cpu = "100m",
								memory = "2G",
								diskSize = "5Gi",
							},
							audit = {
								enabled = true,
							},
						},
						{
							name = "without-audit",
							majorVersion = "15",
							resources = {
								cpu = "100m",
								memory = "1G",
								diskSize = "3Gi",
							},
							audit = {
								enabled = false,
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("Get specific postgres instance from team environment", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		{
		  team(slug: "someteamname") {
		    environment(name: "dev") {
		      postgresInstance(name: "foobar") {
		        name
		        majorVersion
		        teamEnvironment {
		          name
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
					postgresInstance = {
						name = "foobar",
						majorVersion = "17",
						teamEnvironment = {
							name = "dev",
						},
					},
				},
			},
		},
	}
end)

Test.gql("List postgres instances with ordering by name", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		{
		  team(slug: "someteamname") {
		    postgresInstances(orderBy: {field: NAME, direction: ASC}) {
		      nodes {
		        name
		      }
		    }
		  }
		}
	]]

	t.check {
		data = {
			team = {
				postgresInstances = {
					nodes = {
						{
							name = "another-db",
						},
						{
							name = "foobar",
						},
						{
							name = "with-audit",
						},
						{
							name = "without-audit",
						},
					},
				},
			},
		},
	}
end)

Test.gql("List postgres instances with ordering by environment", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		{
		  team(slug: "someteamname") {
		    postgresInstances(orderBy: {field: ENVIRONMENT, direction: DESC}, first: 10) {
		      nodes {
		        name
		        teamEnvironment {
		          name
		        }
		      }
		    }
		  }
		}
	]]

	t.check {
		data = {
			team = {
				postgresInstances = {
					nodes = {
						{
							name = "another-db",
							teamEnvironment = {
								name = "dev",
							},
						},
						{
							name = "foobar",
							teamEnvironment = {
								name = "dev",
							},
						},
						{
							name = "with-audit",
							teamEnvironment = {
								name = "dev",
							},
						},
						{
							name = "without-audit",
							teamEnvironment = {
								name = "dev",
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("Get postgres instance from application", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		{
		  team(slug: "someteamname") {
		    environment(name: "dev") {
		      application(name: "app-with-postgres") {
		        name
		        postgresInstances {
		          nodes {
		            name
		            majorVersion
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
						name = "app-with-postgres",
						postgresInstances = {
							nodes = {
								{
									name = "foobar",
									majorVersion = "17",
								},
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("Get empty postgres instances from application without postgres", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		{
		  team(slug: "someteamname") {
		    environment(name: "dev") {
		      application(name: "app-without-postgres") {
		        name
		        postgresInstances {
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
				environment = {
					application = {
						name = "app-without-postgres",
						postgresInstances = {
							nodes = {},
						},
					},
				},
			},
		},
	}
end)

Test.gql("Access postgres instance fields", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		{
		  team(slug: "someteamname") {
		    postgresInstances(first: 1) {
		      nodes {
		        id
		        name
		        majorVersion
		        team {
		          slug
		        }
		        teamEnvironment {
		          name
		        }
		        resources {
		          cpu
		          memory
		          diskSize
		        }
		      }
		    }
		  }
		}
	]]

	t.check {
		data = {
			team = {
				postgresInstances = {
					nodes = {
						{
							id = NotNull(),
							name = NotNull(),
							majorVersion = NotNull(),
							team = {
								slug = "someteamname",
							},
							teamEnvironment = {
								name = NotNull(),
							},
							resources = {
								cpu = NotNull(),
								memory = NotNull(),
								diskSize = NotNull(),
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("Postgres instance with audit logging enabled", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		{
		  team(slug: "someteamname") {
		    environment(name: "dev") {
		      postgresInstance(name: "with-audit") {
		        name
		        audit {
		          enabled
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
					postgresInstance = {
						name = "with-audit",
						audit = {
							enabled = true,
						},
					},
				},
			},
		},
	}
end)

Test.gql("Postgres instance without audit logging", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		{
		  team(slug: "someteamname") {
		    environment(name: "dev") {
		      postgresInstance(name: "foobar") {
		        name
		        audit {
		          enabled
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
					postgresInstance = {
						name = "foobar",
						audit = {
							enabled = false,
						},
					},
				},
			},
		},
	}
end)

Test.gql("Postgres instance with explicit audit disabled", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		{
		  team(slug: "someteamname") {
		    environment(name: "dev") {
		      postgresInstance(name: "without-audit") {
		        name
		        audit {
		          enabled
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
					postgresInstance = {
						name = "without-audit",
						audit = {
							enabled = false,
						},
					},
				},
			},
		},
	}
end)
