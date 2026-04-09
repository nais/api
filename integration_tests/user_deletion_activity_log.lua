-- Test the user deletion trigger that creates activity log entries

local userToDelete = User.new("user_to_delete", "delete@example.com", "delete123")
local team1 = Team.new("team-deletion-log-1", "purpose", "#channel")
local team2 = Team.new("team-deletion-log-2", "purpose", "#channel")
local otherUser = User.new("other_user", "other@example.com", "other123")

-- Add user to two teams with different roles
team1:addMember(userToDelete)
team1:addOwner(otherUser)

team2:addOwner(userToDelete)
team2:addOwner(otherUser)

Test.gql("Check activity log before user deletion", function(t)
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
	]], "team-deletion-log-1"))

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

Test.gql("Check activity log after user deletion for first team", function(t)
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
	]], "team-deletion-log-1"))

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
							resourceName = "team-deletion-log-1",
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

Test.gql("Check activity log after user deletion for second team", function(t)
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
	]], "team-deletion-log-2"))

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
							resourceName = "team-deletion-log-2",
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
