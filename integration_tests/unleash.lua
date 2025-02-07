local teamSlug = "slug-1"

-- Ensure the default user has the role "Team member" for the team "slug-1"
Helper.SQLExec([[
	INSERT INTO
		user_roles (role_name, user_id, target_team_slug)
	VALUES (
		'Team member',
		(SELECT id FROM users WHERE email = 'authenticated@example.com'),
		$1
	)
	ON CONFLICT DO NOTHING;
	;
]], teamSlug)

Test.gql("Get unleash when no instance exists", function(t)
	t.query(string.format([[
		{
			team(slug: "%s") {
				unleash {
					name
				}
			}
		}
	]], teamSlug))

	t.check {
		data = {
			team = {
				unleash = Null,
			},
		},
	}
end)

Test.gql("Create unleash instance", function(t)
	t.query(string.format([[
		mutation {
			createUnleashForTeam(input: {teamSlug: "slug-1"}) {
				unleash {
					name
				}
			}
		}
	]], teamSlug))

	t.check {
		data = {
			createUnleashForTeam = {
				unleash = {
					name = teamSlug,
				},
			},
		},
	}
end)

Test.gql("Get unleash when instance exists", function(t)
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
	]], teamSlug))

	t.check {
		data = {
			team = {
				unleash = {
					name = teamSlug,
					allowedTeams = {
						nodes = {
							{ slug = teamSlug },
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
	]], teamSlug))

	t.check {
		data = {
			allowTeamAccessToUnleash = {
				unleash = {
					name = teamSlug,
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
	]], teamSlug))

	t.check {
		data = {
			team = {
				unleash = {
					name = teamSlug,
					allowedTeams = {
						nodes = {
							{ slug = teamSlug },
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
	]], teamSlug))

	t.check {
		data = {
			revokeTeamAccessToUnleash = {
				unleash = {
					name = teamSlug,
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
	]], teamSlug))

	t.check {
		data = {
			team = {
				unleash = {
					name = teamSlug,
					allowedTeams = {
						nodes = {
							{ slug = teamSlug },
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
	t.check("unleash.nais.io/v1", "unleashes", "management", "bifrost-unleash", teamSlug, {
		apiVersion = "unleash.nais.io/v1",
		kind = "Unleash",
		metadata = {
			creationTimestamp = Ignore(),
			name = teamSlug,
			namespace = "bifrost-unleash",
		},
		spec = {
			apiIngress = {
				host = teamSlug .. "-unleash-api.example.com",
			},
			database = {},
			extraEnvVars = {
				{
					name = "TEAMS_ALLOWED_TEAMS",
					value = teamSlug,
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
				host = teamSlug .. "-unleash-web.example.com",
			},
		},
		status = Ignore(), -- This is mocked in the test
	})
end)
