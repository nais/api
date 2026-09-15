Helper.readK8sResources("./k8s_resources/valkey_version")
local user = User.new()
local team = Team.new("versionteam", "purpose", "#channel")

-- One fixture per way the desired version is arrived at. "oldrunning" records nothing, so it has
-- to come from Aiven rather than from the fallback. "pinned" records a version Aiven has already
-- moved past, so the adopted running version wins over the stale spec value. "pending-oldrunning"
-- records a version ahead of what Aiven runs, so the two must stay distinct.
Test.gql("Show version of Valkey instances", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format(
		[[
{
  team(slug: "%s") {
    valkeys {
      nodes {
        name
        version {
          actual
          desiredMajor
        }
      }
    }
  }
}]],
		team:slug()
	))

	t.check {
		data = {
			team = {
				valkeys = {
					nodes = {
						{
							name = "oldrunning",
							version = {
								actual = "8.1.2",
								desiredMajor = "V8_1",
							},
						},
						{
							name = "pending-oldrunning",
							version = {
								actual = "8.1.2",
								desiredMajor = "V9_1",
							},
						},
						{
							name = "pinned",
							version = {
								actual = "9.1.0",
								desiredMajor = "V9_1",
							},
						},
					},
				},
			},
		},
	}
end)
