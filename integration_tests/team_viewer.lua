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
				userIsMember
				userIsOwner
			}

			team2: team(slug:"owner") {
				userIsMember
				userIsOwner
			}

			team3: team(slug:"not-a-member") {
				userIsMember
				userIsOwner
			}
		}
	]]

	t.check {
		data = {
			team1 = {
				userIsMember = true,
				userIsOwner = false,
			},
			team2 = {
				userIsMember = true,
				userIsOwner = true,
			},
			team3 = {
				userIsMember = false,
				userIsOwner = false,
			},
		},
	}
end)
