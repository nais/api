local teamSlug = "slug-1"
local user = User.new("original_user", "orig@team.com", "o")

local memberToAdd = User.new("member", "member@team.com", "3")
local ownerToAdd = User.new("owner", "owner@team.com", "4")

Test.gql("Create team", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		mutation {
			createTeam(
				input: {
					slug: "%s",
					purpose: "some purpose",
					slackChannel: "#channel"
				}
			) {
				team {
					slug
				}
			}
		}
	]], teamSlug))

	t.check {
		data = {
			createTeam = {
				team = {
					slug = teamSlug,
				},
			},
		},
	}
end)

Test.gql("Set role on user that is not a member", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		mutation {
			setTeamMemberRole(
				input: {
					teamSlug: "%s",
					userEmail: "%s",
					role: EDITOR
				}
			) {
				member {
					role
				}
			}
		}
	]], teamSlug, memberToAdd:email()))

	t.check {
		data = Null,
		errors = {
			{
				message = "User is not a member of the team.",
				path = {
					"setTeamMemberRole",
				},
			},
		},
	}
end)

Test.gql("Add user that does not exist", function(t)
	t.addHeader("x-user-email", user:email())

	Helper.emptyPubSubTopic("topic")

	t.query(string.format([[
		mutation {
			addTeamMember(
				input: {
					teamSlug: "%s"
					userEmail: "userthatdoesnotexist@example.com"
					role: EDITOR
				}
			) {
				member {
					role
				}
			}
		}
	]], teamSlug))

	t.check {
		data = Null,
		errors = {
			{
				message = "The specified user was not found.",
				path = {
					"addTeamMember",
				},
			},
		},
	}
end)

Test.gql("Add member", function(t)
	t.addHeader("x-user-email", user:email())

	Helper.emptyPubSubTopic("topic")

	t.query(string.format([[
		mutation {
			addTeamMember(
				input: {
					teamSlug: "%s"
					userEmail: "%s"
					role: EDITOR
				}
			) {
				member {
					role
					team {
						activityLog(first: 5) {
							nodes {
								__typename
								message
								actor
								resourceType
								resourceName
								... on TeamMemberAddedActivityLogEntry {
									data {
										role
										userEmail
										userID
									}
								}
							}
						}
					}
				}
			}
		}
	]], teamSlug, memberToAdd:email()))

	t.check {
		data = {
			addTeamMember = {
				member = {
					role = "EDITOR",
					team = {
						activityLog = {
							nodes = {
								{
									__typename = "TeamMemberAddedActivityLogEntry",
									message = "Add member",
									actor = user:email(),
									resourceType = "TEAM",
									resourceName = teamSlug,
									data = {
										role = "EDITOR",
										userEmail = memberToAdd:email(),
										userID = NotNull(),
									},
								},
								{
									__typename = "TeamCreatedActivityLogEntry",
									message = "Created team",
									actor = user:email(),
									resourceType = "TEAM",
									resourceName = teamSlug,
								},
							},
						},
					},
				},
			},
		},
	}
end)

Test.pubsub("Check if pubsub message was sent", function(t)
	t.check("topic", {
		attributes = {
			CorrelationID = NotNull(),
			EventType = "EVENT_TEAM_UPDATED",
		},
		data = {
			slug = teamSlug,
		},
	})
end)

Test.gql("Change role", function(t)
	t.addHeader("x-user-email", user:email())

	Helper.emptyPubSubTopic("topic")

	t.query(string.format([[
		mutation {
			setTeamMemberRole(
				input: {
					teamSlug: "%s",
					userEmail: "%s",
					role: OWNER
				}
			) {
				member {
					role
					team {
						activityLog(first: 5) {
							nodes {
								__typename
								message
								actor
								resourceType
								resourceName
								... on TeamMemberSetRoleActivityLogEntry {
									data {
										role
										userEmail
										userID
									}
								}
							}
						}
					}
				}
			}
		}
	]], teamSlug, memberToAdd:email()))

	t.check {
		data = {
			setTeamMemberRole = {
				member = {
					role = "OWNER",
					team = {
						activityLog = {
							nodes = {
								{
									__typename = "TeamMemberSetRoleActivityLogEntry",
									message = "Set member role",
									actor = user:email(),
									resourceType = "TEAM",
									resourceName = teamSlug,
									data = {
										role = "OWNER",
										userEmail = memberToAdd:email(),
										userID = NotNull(),
									},
								},
								{
									__typename = "TeamMemberAddedActivityLogEntry",
									message = "Add member",
									actor = user:email(),
									resourceType = "TEAM",
									resourceName = teamSlug,
								},
								{
									__typename = "TeamCreatedActivityLogEntry",
									message = "Created team",
									actor = user:email(),
									resourceType = "TEAM",
									resourceName = teamSlug,
								},
							},
						},
					},
				},
			},
		},
	}
end)

Test.pubsub("Check if pubsub message was sent after role change", function(t)
	t.check("topic", {
		attributes = {
			CorrelationID = NotNull(),
			EventType = "EVENT_TEAM_UPDATED",
		},
		data = {
			slug = teamSlug,
		},
	})
end)

Test.gql("Add owner", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		mutation {
			addTeamMember(
				input: {
					teamSlug: "%s"
					userEmail: "%s"
					role: OWNER
				}
			) {
				member {
					role
					team {
						members {
							nodes {
								role
								user {
									email
									name
								}
							}
						}
					}
				}
			}
		}
	]], teamSlug, ownerToAdd:email()))

	t.check {
		data = {
			addTeamMember = {
				member = {
					role = "OWNER",
					team = {
						members = {
							nodes = {
								{
									role = "OWNER",
									user = {
										email = memberToAdd:email(),
										name = memberToAdd:name(),
									},
								},
								{
									role = "OWNER",
									user = {
										email = user:email(),
										name = user:name(),
									},
								},
								{
									role = "OWNER",
									user = {
										email = ownerToAdd:email(),
										name = ownerToAdd:name(),
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

Test.gql("Remove owner", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		mutation {
			removeTeamMember(
				input: {
					teamSlug: "%s"
					userEmail: "%s"
				}
			) {
				team {
					members {
						nodes {
							role
							user {
								email
								name
							}
						}
					}
					activityLog(first: 5) {
						nodes {
							__typename
							message
							actor
							resourceType
							resourceName
							... on TeamMemberRemovedActivityLogEntry {
								data {
									userEmail
									userID
								}
							}
						}
					}
				}
			}
		}
	]], teamSlug, ownerToAdd:email()))

	t.check {
		data = {
			removeTeamMember = {
				team = {
					members = {
						nodes = {
							{
								role = "OWNER",
								user = {
									email = memberToAdd:email(),
									name = memberToAdd:name(),
								},
							},
							{
								role = "OWNER",
								user = {
									email = user:email(),
									name = user:name(),
								},
							},
						},
					},
					activityLog = {
						nodes = {
							{
								__typename = "TeamMemberRemovedActivityLogEntry",
								message = "Remove member",
								actor = user:email(),
								resourceType = "TEAM",
								resourceName = teamSlug,
								data = {
									userEmail = ownerToAdd:email(),
									userID = NotNull(),
								},
							},
							{
								__typename = "TeamMemberAddedActivityLogEntry",
								message = "Add member",
								actor = user:email(),
								resourceType = "TEAM",
								resourceName = teamSlug,
							},
							{
								__typename = "TeamMemberSetRoleActivityLogEntry",
								message = "Set member role",
								actor = user:email(),
								resourceType = "TEAM",
								resourceName = teamSlug,
							},
							{
								__typename = "TeamMemberAddedActivityLogEntry",
								message = "Add member",
								actor = user:email(),
								resourceType = "TEAM",
								resourceName = teamSlug,
							},
							{
								__typename = "TeamCreatedActivityLogEntry",
								message = "Created team",
								actor = user:email(),
								resourceType = "TEAM",
								resourceName = teamSlug,
							},
						},
					},
				},
			},
		},
	}
end)
