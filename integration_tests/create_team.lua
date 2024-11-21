Test.gql("Create team", function(t)
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

	local invalidPrefix = {
		"team",
		"teamfoo",
		"team-foo",
	}
	testSlug(invalidPrefix, Contains("The name prefix 'team' is redundant."))

	local shortSlugs = {
		"a",
		"ab",
	}
	testSlug(shortSlugs, "A team slug must be at least 3 characters long.")

	local longSlugs = {
		"some-long-string-more-than-30-chars",
	}
	testSlug(longSlugs, "A team slug must be at most 30 characters long.")

	local reservedSlugs = {
		"nais-system",
		"kube-system",
		"kube-node-lease",
		"kube-public",
		"kyverno",
		"cnrm-system",
		"configconnector-operator-system",
	}
	testSlug(reservedSlugs, "This slug is reserved by NAIS.")

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
		azure_group_id = Null,
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
