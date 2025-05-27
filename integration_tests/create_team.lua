Helper.readK8sResources("./k8s_resources/simple")

local user = User.new()
local unauthorized = User.new()
local existingTeam = Team.new("slug-1", "purpose", "#channel")

Helper.SQLExec([[
	DELETE FROM user_roles WHERE user_id = $1
]], unauthorized:id())

Test.gql("Create team with user that is not authorized", function(t)
	t.addHeader("x-user-email", unauthorized:email())

	t.query([[
		mutation {
			createTeam(
				input: {
					slug: "slug-1"
					purpose: "some purpose"
					slackChannel: "#channel"
				}
			) {
				team {
					id
				}
			}
		}
	]])

	t.check {
		data = Null,
		errors = {
			{
				message = Contains("Specifically, you need the \"teams:create\" authorization."),
				path = {
					"createTeam",
				},
			},
		},
	}
end)

Test.gql("Create team with namespace that already exists", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			createTeam(
				input: {
					slug: "slug-1"
					purpose: "some purpose"
					slackChannel: "#channel"
				}
			) {
				team {
					id
				}
			}
		}
	]]

	t.check {
		data = Null,
		errors = {
			{
				extensions = {
					field = "slug",
				},
				message = "Team slug is not available.",
				path = {
					"createTeam",
				},
			},
		},
	}
end)

Test.gql("Create team", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			createTeam(
				input: {
					slug: "newteam"
					purpose: "some purpose"
					slackChannel: "#channel"
				}
			) {
				team {
					id
					slug
				}
			}
		}
	]]

	t.check {
		data = {
			createTeam = {
				team = {
					id = Save("teamID"),
					slug = "newteam",
				},
			},
		},
	}
end)

Test.gql("Create team with invalid slug", function(t)
	t.addHeader("x-user-email", user:email())

	local testSlug = function(slugs, errorMessageMatch)
		for _, s in ipairs(slugs) do
			t.query(string.format([[
				mutation {
					createTeam(
						input: {
							slug: "%s"
							purpose: "some purpose"
							slackChannel: "#channel"
						}
					) {
						team {
							id
							slug
						}
					}
				}
			]], s))
			t.check {
				data = Null,
				errors = {
					{
						message = errorMessageMatch,
						path = {
							"createTeam",
							"input",
							"slug",
						},
					},
				},
			}
		end
	end

	local testSlugWithTeamPrefix = function(slugs, errorMessageMatch)
		for _, s in ipairs(slugs) do
			t.query(string.format([[
				mutation {
					createTeam(
						input: {
							slug: "%s"
							purpose: "some purpose"
							slackChannel: "#channel"
						}
					) {
						team {
							id
							slug
						}
					}
				}
			]], s))
			t.check {
				data = Null,
				errors = {
					{
						message = errorMessageMatch,
						path = {
							"createTeam",
						},
					},
				},
			}
		end
	end

	local invalidPrefix = {
		"team",
		"teamfoo",
		"team-foo",
	}
	testSlugWithTeamPrefix(invalidPrefix, Contains("The name prefix 'team' is redundant."))



	local reservedPrefix = {
		"naisteam",
		"nais-foo",
		"pg-foo",
	}
	testSlugWithTeamPrefix(reservedPrefix, Contains("is reserved."))

	local shortSlugs = {
		"a",
		"ab",
	}
	testSlug(shortSlugs, "A team slug must be at least 3 characters long.")

	local longSlugs = {
		"some-long-string-more-than-30-chars",
	}
	testSlug(longSlugs, "A team slug must be at most 30 characters long.")

	local invalidSlugs = {
		"-foo",
		"foo-",
		"foo--bar",
		"4chan",
		"you-aint-got-the-æøå",
		"Uppercase",
		"rollback()",
	}
	testSlug(invalidSlugs, Contains("A team slug must match the following pattern:"))
end)

Test.gql("Create team with invalid Slack channel name", function(t)
	t.addHeader("x-user-email", user:email())

	local invalidSlackChannelNames = {
		"foo",                                                                          -- missing hash
		"#a",                                                                           -- too short
		"#aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaabbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", -- too long
		"#foo bar",                                                                     -- space not allowed
		"#Foobar",                                                                      -- upper case not allowed
	}
	for _, s in ipairs(invalidSlackChannelNames) do
		t.query(string.format([[
			mutation {
				createTeam(
					input: {
						slug: "myteam"
						purpose: "some purpose"
						slackChannel: "%s"
					}
				) {
					team {
						id
						slug
					}
				}
			}
		]], s))
		t.check {
			data = Null,
			errors = {
				{
					message = Contains("The Slack channel does not fit the requirements."),
					extensions = {
						field = "slackChannel",
					},
					path = {
						"createTeam",
					},
				},
			},
		}
	end
end)

Test.pubsub("Check if pubsub message was sent", function(t)
	t.check("topic", {
		attributes = {
			CorrelationID = NotNull(),
			EventType = "EVENT_TEAM_CREATED",
		},
		data = {
			slug = "newteam",
		},
	})
end)

Test.sql("Check database", function(t)
	t.queryRow("SELECT * FROM teams WHERE slug = $1", "newteam")

	t.check {
		entra_id_group_id = Null,
		gar_repository = Null,
		github_team_slug = Null,
		google_group_email = Null,
		last_successful_sync = Null,
		cdn_bucket = Null,
		delete_key_confirmed_at = Null,
		purpose = "some purpose",
		slack_channel = "#channel",
		slug = "newteam",
	}
end)

Test.gql("Team node query", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		query {
			node(id: "%s") {
				... on Team {
					slug
				}
			}
		}
	]], State.teamID))

	t.check {
		data = {
			node = {
				slug = "newteam",
			},
		},
	}
end)

Test.gql("Create team with unavailable slug", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			createTeam(
				input: {
					slug: "newteam"
					purpose: "some purpose"
					slackChannel: "#channel"
				}
			) {
				team {
					id
					slug
				}
			}
		}
	]]

	t.check {
		data = Null,
		errors = {
			{
				extensions = {
					field = "slug",
				},
				message = "Team slug is not available.",
				path = {
					"createTeam",
				},
			},
		},
	}
end)
