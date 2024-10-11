TeamSlug = "myteam"
MemberToAdd = "email-1@example.com"
OwnerToAdd = "email-2@example.com"

Test.gql("Create team", function(t)
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
	]], TeamSlug))

	t.check {
		data = {
			createTeam = {
				team = {
					slug = TeamSlug
				}
			}
		}
	}
end)

Test.gql("Set role on user that is not a member", function(t)
	t.query(string.format([[
		mutation {
			setTeamMemberRole(
				input: {
					teamSlug: "%s",
					userEmail: "%s",
					role: MEMBER
				}
			) {
				member {
					role
				}
			}
		}
	]], TeamSlug, MemberToAdd))

	t.check {
		data = Null,
		errors = {
			{
				message = "User is not a member of the team.",
				path = {
					"setTeamMemberRole"
				}
			}
		}
	}
end)

Test.gql("Add user that does not exist", function(t)
	Helper.emptyPubSubTopic("topic")

	t.query(string.format([[
		mutation {
			addTeamMember(
				input: {
					teamSlug: "%s"
					userEmail: "userthatdoesnotexist@example.com"
					role: MEMBER
				}
			) {
				member {
					role
				}
			}
		}
	]], TeamSlug))

	t.check {
		data = Null,
		errors = {
			{
				message = "The specified user was not found.",
				path = {
					"addTeamMember"
				}
			}
		}
	}
end)

Test.gql("Add member", function(t)
	Helper.emptyPubSubTopic("topic")

	t.query(string.format([[
		mutation {
			addTeamMember(
				input: {
					teamSlug: "%s"
					userEmail: "%s"
					role: MEMBER
				}
			) {
				member {
					role
				}
			}
		}
	]], TeamSlug, MemberToAdd))

	t.check {
		data = {
			addTeamMember = {
				member = {
					role = "MEMBER"
				}
			}
		}
	}
end)

Test.pubsub("Check if pubsub message was sent", function(t)
	t.check("topic", {
		attributes = {
			CorrelationID = NotNull(),
			EventType = "EVENT_TEAM_UPDATED"
		},
		data = {
			slug = TeamSlug
		}
	})
end)

Test.gql("Change role", function(t)
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
				}
			}
		}
	]], TeamSlug, MemberToAdd))

	t.check {
		data = {
			setTeamMemberRole = {
				member = {
					role = "OWNER"
				}
			}
		}
	}
end)

Test.pubsub("Check if pubsub message was sent", function(t)
	t.check("topic", {
		attributes = {
			CorrelationID = NotNull(),
			EventType = "EVENT_TEAM_UPDATED"
		},
		data = {
			slug = TeamSlug
		}
	})
end)

Test.gql("Add owner", function(t)
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
	]], TeamSlug, OwnerToAdd))

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
										email = "authenticated@example.com",
										name = "Authenticated User"
									}
								},
								{
									role = "OWNER",
									user = {
										email = MemberToAdd,
										name = "name-1"
									}
								},
								{
									role = "OWNER",
									user = {
										email = OwnerToAdd,
										name = "name-2"
									}
								}
							}
						}
					}
				}
			}
		}
	}
end)
