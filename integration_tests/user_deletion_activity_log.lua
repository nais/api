-- Test the user deletion trigger that creates activity log entries

local userToDelete = User.new("user_to_delete", "delete@example.com", "delete123")
local teamWithSingleRole = Team.new("team-single-role", "purpose", "#channel")
local teamWithMultipleRoles = Team.new("team-multiple-roles", "purpose", "#channel")
local otherUser = User.new("other_user", "other@example.com", "other123")

-- Add user to first team with single role
teamWithSingleRole:addMember(userToDelete)
teamWithSingleRole:addOwner(otherUser)

-- Add user to second team with multiple roles (to test deduplication)
teamWithMultipleRoles:addMember(userToDelete)
teamWithMultipleRoles:addOwner(userToDelete)
teamWithMultipleRoles:addOwner(otherUser)

Test.gql("Check activity log before user deletion for team with single role", function(t)
	t.addHeader("x-user-email", otherUser:email())

	t.query(string.format([[
		{
			team(slug: "%s") {
				activityLog(first: 10) {
					nodes {
						__typename
						message
					}
				}
			}
		}
	]], "team-single-role"))

	t.check {
		data = {
			team = {
				activityLog = {
					nodes = {},
				},
			},
		},
	}
end)

-- Delete the user using SQL - this triggers the log_user_deletion() function
Helper.SQLExec("DELETE FROM users WHERE id = $1", userToDelete:id())

Test.gql("Check activity log after user deletion for team with single role", function(t)
	t.addHeader("x-user-email", otherUser:email())

	t.query(string.format([[
		{
			team(slug: "%s") {
				activityLog(first: 10, filter: { activityTypes: [TEAM_MEMBER_REMOVED] }) {
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
	]], "team-single-role"))

	t.check {
		data = {
			team = {
				activityLog = {
					nodes = {
						{
							__typename = "TeamMemberRemovedActivityLogEntry",
							message = "Remove member",
							actor = "system",
							resourceType = "TEAM",
							resourceName = "team-single-role",
							data = {
								userEmail = userToDelete:email(),
								userID = NotNull(),
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("Check activity log after user deletion for team with multiple roles (deduplication)", function(t)
	t.addHeader("x-user-email", otherUser:email())

	t.query(string.format([[
		{
			team(slug: "%s") {
				activityLog(first: 10, filter: { activityTypes: [TEAM_MEMBER_REMOVED] }) {
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
	]], "team-multiple-roles"))

	t.check {
		data = {
			team = {
				activityLog = {
					nodes = {
						{
							__typename = "TeamMemberRemovedActivityLogEntry",
							message = "Remove member",
							actor = "system",
							resourceType = "TEAM",
							resourceName = "team-multiple-roles",
							data = {
								userEmail = userToDelete:email(),
								userID = NotNull(),
							},
						},
					},
				},
			},
		},
	}
end)
