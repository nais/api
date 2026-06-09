local user = User.new("user", "user@usersen.com")
local team = Team.new("labelteam", "purpose", "#slack_channel")
team:addMember(user)

Helper.readK8sResources("k8s_resources/label_selectors")

Test.gql("Check all Valkey instances (no filter)", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		{
			team(slug: "labelteam") {
				slug
				valkeys {
					pageInfo {
						totalCount
					}
					nodes {
						name
						labels {
							key
							value
						}
					}
				}
			}
		}
	]]

	t.check {
		data = {
			team = {
				slug = "labelteam",
				valkeys = {
					pageInfo = {
						totalCount = 3,
					},
					nodes = {
						{
							name = "valkey-one",
							labels = {
								{ key = "labels.nais.io/priority", value = "high" },
								{ key = "labels.nais.io/tag",      value = "target" },
							},
						},
						{
							name = "valkey-three",
							labels = {
								{ key = "labels.nais.io/tag", value = "other" },
							},
						},
						{
							name = "valkey-two",
							labels = {
								{ key = "labels.nais.io/tag", value = "target" },
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("Valkey filter by tag=target", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		{
			team(slug: "labelteam") {
				valkeys(filter: { labels: [{ key: "labels.nais.io/tag", value: "target" }] }) {
					pageInfo {
						totalCount
					}
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
				valkeys = {
					pageInfo = {
						totalCount = 2,
					},
					nodes = {
						{ name = "valkey-one" },
						{ name = "valkey-two" },
					},
				},
			},
		},
	}
end)

Test.gql("Valkey filter by tag=target and priority=high", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		{
			team(slug: "labelteam") {
				valkeys(filter: {
					labels: [
						{ key: "labels.nais.io/tag", value: "target" },
						{ key: "labels.nais.io/priority", value: "high" }
					]
				}) {
					pageInfo {
						totalCount
					}
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
				valkeys = {
					pageInfo = {
						totalCount = 1,
					},
					nodes = {
						{ name = "valkey-one" },
					},
				},
			},
		},
	}
end)

Test.gql("Check all Postgres instances (no filter)", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		{
			team(slug: "labelteam") {
				slug
				postgresInstances {
					pageInfo {
						totalCount
					}
					nodes {
						name
						labels {
							key
							value
						}
					}
				}
			}
		}
	]]

	t.check {
		data = {
			team = {
				slug = "labelteam",
				postgresInstances = {
					pageInfo = {
						totalCount = 3,
					},
					nodes = {
						{
							name = "postgres-one",
							labels = {
								{ key = "labels.nais.io/priority", value = "high" },
								{ key = "labels.nais.io/tag",      value = "target" },
							},
						},
						{
							name = "postgres-three",
							labels = {
								{ key = "labels.nais.io/tag", value = "other" },
							},
						},
						{
							name = "postgres-two",
							labels = {
								{ key = "labels.nais.io/tag", value = "target" },
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("Postgres filter by tag=target", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		{
			team(slug: "labelteam") {
				postgresInstances(filter: { labels: [{ key: "labels.nais.io/tag", value: "target" }] }) {
					pageInfo {
						totalCount
					}
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
					pageInfo = {
						totalCount = 2,
					},
					nodes = {
						{ name = "postgres-one" },
						{ name = "postgres-two" },
					},
				},
			},
		},
	}
end)

Test.gql("Postgres filter by tag=target and priority=high", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		{
			team(slug: "labelteam") {
				postgresInstances(filter: {
					labels: [
						{ key: "labels.nais.io/tag", value: "target" },
						{ key: "labels.nais.io/priority", value: "high" }
					]
				}) {
					pageInfo {
						totalCount
					}
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
					pageInfo = {
						totalCount = 1,
					},
					nodes = {
						{ name = "postgres-one" },
					},
				},
			},
		},
	}
end)

Test.gql("Check all applications (no filter)", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		{
			team(slug: "labelteam") {
				slug
				applications {
					pageInfo {
						totalCount
					}
					nodes {
						name
						labels {
							key
							value
						}
					}
				}
			}
		}
	]]

	t.check {
		data = {
			team = {
				slug = "labelteam",
				applications = {
					pageInfo = {
						totalCount = 3,
					},
					nodes = {
						{
							name = "app-one",
							labels = {
								{ key = "labels.nais.io/priority", value = "high" },
								{ key = "labels.nais.io/tag",      value = "target" },
							},
						},
						{
							name = "app-three",
							labels = {
								{ key = "labels.nais.io/tag", value = "other" },
							},
						},
						{
							name = "app-two",
							labels = {
								{ key = "labels.nais.io/tag", value = "target" },
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("Application filter by tag=target", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		{
			team(slug: "labelteam") {
				applications(filter: { labels: [{ key: "labels.nais.io/tag", value: "target" }] }) {
					pageInfo {
						totalCount
					}
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
				applications = {
					pageInfo = {
						totalCount = 2,
					},
					nodes = {
						{ name = "app-one" },
						{ name = "app-two" },
					},
				},
			},
		},
	}
end)

Test.gql("Application filter by tag=target and priority=high", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		{
			team(slug: "labelteam") {
				applications(filter: {
					labels: [
						{ key: "labels.nais.io/tag", value: "target" },
						{ key: "labels.nais.io/priority", value: "high" }
					]
				}) {
					pageInfo {
						totalCount
					}
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
				applications = {
					pageInfo = {
						totalCount = 1,
					},
					nodes = {
						{ name = "app-one" },
					},
				},
			},
		},
	}
end)

Test.gql("Check all jobs (no filter)", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		{
			team(slug: "labelteam") {
				slug
				jobs {
					pageInfo {
						totalCount
					}
					nodes {
						name
						labels {
							key
							value
						}
					}
				}
			}
		}
	]]

	t.check {
		data = {
			team = {
				slug = "labelteam",
				jobs = {
					pageInfo = {
						totalCount = 3,
					},
					nodes = {
						{
							name = "job-one",
							labels = {
								{ key = "labels.nais.io/priority", value = "high" },
								{ key = "labels.nais.io/tag",      value = "target" },
							},
						},
						{
							name = "job-three",
							labels = {
								{ key = "labels.nais.io/tag", value = "other" },
							},
						},
						{
							name = "job-two",
							labels = {
								{ key = "labels.nais.io/tag", value = "target" },
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("Job filter by tag=target", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		{
			team(slug: "labelteam") {
				jobs(filter: { name: "", labels: [{ key: "labels.nais.io/tag", value: "target" }] }) {
					pageInfo {
						totalCount
					}
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
				jobs = {
					pageInfo = {
						totalCount = 2,
					},
					nodes = {
						{ name = "job-one" },
						{ name = "job-two" },
					},
				},
			},
		},
	}
end)

Test.gql("Job filter by tag=target and priority=high", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		{
			team(slug: "labelteam") {
				jobs(filter: {
					name: ""
					labels: [
						{ key: "labels.nais.io/tag", value: "target" },
						{ key: "labels.nais.io/priority", value: "high" }
					]
				}) {
					pageInfo {
						totalCount
					}
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
				jobs = {
					pageInfo = {
						totalCount = 1,
					},
					nodes = {
						{ name = "job-one" },
					},
				},
			},
		},
	}
end)

Test.gql("Valkey filter with invalid label prefix", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		{
			team(slug: "labelteam") {
				valkeys(filter: { labels: [{ key: "tag", value: "target" }] }) {
					pageInfo {
						totalCount
					}
				}
			}
		}
	]]

	t.check {
		errors = {
			{
				message = Contains("label key \"tag\" must be prefixed with \"labels.nais.io/\""),
				path = {
					"team",
					"valkeys",
					"filter",
					"labels",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Check labels facets on Valkey connection", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		{
			team(slug: "labelteam") {
				valkeys {
					facets {
						labels {
							key
							value
							count
						}
					}
				}
			}
		}
	]]

	t.check {
		data = {
			team = {
				valkeys = {
					facets = {
						labels = {
							{ key = "labels.nais.io/priority", value = "high", count = 1 },
							{ key = "labels.nais.io/tag",      value = "other", count = 1 },
							{ key = "labels.nais.io/tag",      value = "target", count = 2 },
						},
					},
				},
			},
		},
	}
end)

