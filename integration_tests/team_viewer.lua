local memberTeam = Team.new("member", "Member", "#member")
local ownerTeam = Team.new("owner", "Owner", "#owner")
local notAMemberTeam = Team.new("not-a-member", "not-a-member", "#not-a-member")

local user = User.new("authenticated", "authenticated@example.com", "some-id")

memberTeam:addMember(user)
ownerTeam:addOwner(user)


Test.gql("Check team is user / owner", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team1: team(slug:"member") {
				viewerIsMember
				viewerIsOwner
			}

			team2: team(slug:"owner") {
				viewerIsMember
				viewerIsOwner
			}

			team3: team(slug:"not-a-member") {
				viewerIsMember
				viewerIsOwner
			}
		}
	]]

	t.check {
		data = {
			team1 = {
				viewerIsMember = true,
				viewerIsOwner = false,
			},
			team2 = {
				viewerIsMember = true,
				viewerIsOwner = true,
			},
			team3 = {
				viewerIsMember = false,
				viewerIsOwner = false,
			},
		},
	}
end)
