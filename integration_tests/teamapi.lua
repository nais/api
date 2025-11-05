local teamName = "my-team"

local pgTeam = Team.new(teamName, "Postgres", "#my-team")
local otherTeam = Team.new("other", "Not Postgres", "#other-team")

local user1 = User.new("user.one", "user.one@example.com", "some-id-1")
local user2 = User.new("user.two", "user.two@example.com", "some-id-2")
local not_a_member = User.new("not.a.member", "not.a.member@example.com", "some-id-3")

pgTeam:addOwner(user1)
pgTeam:addMember(user2)

otherTeam:addOwner(not_a_member)


Test.rest("list team members", function(t)
	t.send("GET", string.format("/teams/%s", teamName))
	t.check(200, {
		member = {
			"some-id-1",
			"some-id-2",
		},
	})
end)

Test.rest("list unknown team", function(t)
	t.send("GET", "/teams/no-such-team")
	t.check(404, {
		errorMessage = "team not found",
		status = 404,
	})
end)


Test.rest("invalid team slug", function(t)
	t.send("GET", "/teams/invalid-ø-slug")
	t.check(400, {
		errorMessage = "A team slug must match the following pattern: \"^[a-z](-?[a-z0-9]+)+$\".",
		status = 400,
	})
end)
