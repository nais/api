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
			releaseChannel = {},
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

Test.gql("Get release channels", function(t)
	t.addHeader("x-user-email", user:email())

	t.query([[
		{
			unleashReleaseChannels {
				name
				currentVersion
				type
				description
			}
		}
	]])

	t.check {
		data = {
			unleashReleaseChannels = {
				{
					name = "stable",
					currentVersion = "5.11.0",
					type = "sequential",
					description = "Stable release channel with tested versions",
				},
				{
					name = "rapid",
					currentVersion = "5.12.0-beta.1",
					type = "canary",
					description = "Rapid release channel with latest features",
				},
				{
					name = "regular",
					currentVersion = "5.10.2",
					type = "sequential",
					description = "Regular release channel with conservative updates",
				},
			},
		},
	}
end)

Test.gql("Update unleash instance with release channel", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		mutation {
			updateUnleashInstance(input: {teamSlug: "%s", releaseChannel: "stable"}) {
				unleash {
					name
					releaseChannelName
				}
			}
		}
	]], team:slug()))

	t.check {
		data = {
			updateUnleashInstance = {
				unleash = {
					name = team:slug(),
					releaseChannelName = "stable",
				},
			},
		},
	}
end)

Test.gql("Update unleash instance validation - both options fail", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		mutation {
			updateUnleashInstance(input: {teamSlug: "%s", customVersion: "5.11.0", releaseChannel: "stable"}) {
				unleash {
					name
				}
			}
		}
	]], team:slug()))

	t.check {
		errors = {
			{
				message = "Cannot specify both customVersion and releaseChannel. These options are mutually exclusive.",
				path = { "updateUnleashInstance" },
				extensions = { field = "customVersion" },
			},
			{
				message = "Cannot specify both customVersion and releaseChannel. These options are mutually exclusive.",
				path = { "updateUnleashInstance" },
				extensions = { field = "releaseChannel" },
			},
		},
		data = Null,
	}
end)

Test.gql("Update unleash instance validation - neither option fails", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		mutation {
			updateUnleashInstance(input: {teamSlug: "%s"}) {
				unleash {
					name
				}
			}
		}
	]], team:slug()))

	t.check {
		errors = {
			{
				message = "Must specify either customVersion or releaseChannel.",
				path = { "updateUnleashInstance" },
				extensions = { field = "customVersion" },
			},
		},
		data = Null,
	}
end)
