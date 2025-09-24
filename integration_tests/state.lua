Helper.readK8sResources("k8s_resources/state")
local user = User.new("name", "auth@user.com", "sdf")
Team.new("myteam", "purpose", "#slack_channel")

local function stateQuery(slug, env, appName)
	return string.format([[
		query {
			team(slug: "%s") {
				environment(name: "%s") {
					application(name: "%s") {
			    	    state
					}
				}
			}
		}
	]], slug, env, appName)
end

Test.gql("app with no instances should not be running", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(stateQuery("myteam", "dev", "no-instances-app"))

	t.check {
		data = {
			team = {
				environment = {
					application = {
						state = "NOT_RUNNING",
					},
				},
			},
		},
	}
end)

Test.gql("failing app should not be running", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(stateQuery("myteam", "dev", "failing-app"))

	t.check {
		data = {
			team = {
				environment = {
					application = {
						state = "NOT_RUNNING",
					},
				},
			},
		},
	}
end)

Test.gql("app with healthy instances is running", function(t)
	t.addHeader("x-user-email", user:email())
	t.query(stateQuery("myteam", "dev", "running-app"))

	t.check {
		data = {
			team = {
				environment = {
					application = {
						state = "RUNNING",
					},
				},
			},
		},
	}
end)
