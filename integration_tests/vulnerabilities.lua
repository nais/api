Team.new("slug-1", "purpose", "#slack_channel")
local memberTeam = Team.new("member", "Member", "#member")
local ownerTeam = Team.new("owner", "Owner", "#owner")
local notAMemberTeam = Team.new("not-a-member", "not-a-member", "#not-a-member")

local user = User.new("authenticated", "authenticated@example.com", "some-id")

memberTeam:addMember(user)
ownerTeam:addOwner(user)

Test.gql("VulnerabilitySummary for Team", function(t)
	t.addHeader("x-user-email", user:email())
	t.query [[
		query {
		  team(slug: "slug-1") {
			workloads{
			  nodes{
				image{
				  vulnerabilitySummary{
					total
					critical
					high
					medium
					low
					unassigned
				  }
				}
			  }
			}
		  }
		}
	]]

	t.check {
		data = {
			team = {
				workloads = {
					nodes = {
						image = {
							vulnerabilitySummary = {
								total = 1,
								critical = 0,
								high = 0,
								medium = 0,
								low = 1,
								unassigned = 0,
								risk_score = 0,
							},
						},
					},
					pageInfo = {
						totalCount = 2,
					},
				},
			},
		},
	}
end)
