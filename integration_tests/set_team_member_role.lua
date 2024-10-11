TeamSlug = "myteam"
MemberToAdd = "email-1@example.com"

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

Test.gql("Add member", function(t)
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

Test.gql("Change role", function(t)
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