Team.new("slug-2", "purpose", "#channel")
local team = Team.new("slug-1", "purpose", "#channel")
local user = User.new()
team:addMember(user)

Test.gql("Get unleash when no instance exists", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		{
			team(slug: "%s") {
				unleash {
					name
				}
			}
		}
	]], team:slug()))

	t.check {
		data = {
			team = {
				unleash = Null,
			},
		},
	}
end)

Test.gql("Create unleash instance", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		mutation {
			createUnleashForTeam(input: {teamSlug: "slug-1"}) {
				unleash {
					name
				}
			}
		}
	]], team:slug()))

	t.check {
		data = {
			createUnleashForTeam = {
				unleash = {
					name = team:slug(),
				},
			},
		},
	}
end)

Test.gql("Get unleash when instance exists", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		{
			team(slug: "%s") {
				unleash {
					name
					allowedTeams {
						nodes {
							slug
						}
						pageInfo {
							totalCount
						}
					}
				}
			}
		}
	]], team:slug()))

	t.check {
		data = {
			team = {
				unleash = {
					name = team:slug(),
					allowedTeams = {
						nodes = {
							{ slug = team:slug() },
						},
						pageInfo = {
							totalCount = 1,
						},
					},
				},
			},
		},
	}
end)

Test.gql("Allow other team to access instance", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		mutation {
			allowTeamAccessToUnleash(input: {teamSlug: "%s", allowedTeamSlug: "slug-2"}) {
				unleash {
					name
					allowedTeams {
						nodes {
							slug
						}
						pageInfo {
							totalCount
						}
					}
				}
			}
		}
	]], team:slug()))

	t.check {
		data = {
			allowTeamAccessToUnleash = {
				unleash = {
					name = team:slug(),
					allowedTeams = {
						nodes = {
							{ slug = "slug-1" },
							{ slug = "slug-2" },
						},
						pageInfo = {
							totalCount = 2,
						},
					},
				},
			},
		},
	}
end)

Test.gql("Get unleash when instance exists after allowing other team", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		{
			team(slug: "%s") {
				unleash {
					name
					allowedTeams {
						nodes {
							slug
						}
						pageInfo {
							totalCount
						}
					}
				}
			}
		}
	]], team:slug()))

	t.check {
		data = {
			team = {
				unleash = {
					name = team:slug(),
					allowedTeams = {
						nodes = {
							{ slug = team:slug() },
							{ slug = "slug-2" },
						},
						pageInfo = {
							totalCount = 2,
						},
					},
				},
			},
		},
	}
end)

Test.gql("Revoke other teams access to instance", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		mutation {
			revokeTeamAccessToUnleash(input: {teamSlug: "%s", revokedTeamSlug: "slug-2"}) {
				unleash {
					name
					allowedTeams {
						nodes {
							slug
						}
						pageInfo {
							totalCount
						}
					}
				}
			}
		}
	]], team:slug()))

	t.check {
		data = {
			revokeTeamAccessToUnleash = {
				unleash = {
					name = team:slug(),
					allowedTeams = {
						nodes = {
							{ slug = "slug-1" },
						},
						pageInfo = {
							totalCount = 1,
						},
					},
				},
			},
		},
	}
end)

Test.gql("Get unleash when instance exists after revoking other team", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		{
			team(slug: "%s") {
				unleash {
					name
					allowedTeams {
						nodes {
							slug
						}
						pageInfo {
							totalCount
						}
					}
				}
			}
		}
	]], team:slug()))

	t.check {
		data = {
			team = {
				unleash = {
					name = team:slug(),
					allowedTeams = {
						nodes = {
							{ slug = team:slug() },
						},
						pageInfo = {
							totalCount = 1,
						},
					},
				},
			},
		},
	}
end)

Test.k8s("Ensure the resource exists", function(t)
	t.check("unleash.nais.io/v1", "unleashes", "management", "bifrost-unleash", team:slug(), {
		apiVersion = "unleash.nais.io/v1",
		kind = "Unleash",
		metadata = {
			creationTimestamp = Ignore(),
			name = team:slug(),
			namespace = "bifrost-unleash",
		},
		spec = {
			apiIngress = {
				host = team:slug() .. "-unleash-api.example.com",
			},
			database = {},
			extraEnvVars = {
				{
					name = "TEAMS_ALLOWED_TEAMS",
					value = team:slug(),
				},
			},
			federation = {},
			networkPolicy = {},
			prometheus = {},
			resources = {
				requests = {
					cpu = "100m",
					memory = "128Mi",
				},
			},
			webIngress = {
				host = team:slug() .. "-unleash-web.example.com",
			},
		},
		status = Ignore(), -- This is mocked in the test
	})
end)
