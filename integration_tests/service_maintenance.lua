local user = User.new()
local teamSlug = "devteam"

Test.gql("Create team", function(t)
	t.addHeader("x-user-email", user:email())

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
	]], teamSlug))

	t.check {
		data = {
			createTeam = {
				team = {
					slug = teamSlug,
				},
			},
		},
	}
end)


Test.gql("Show maintenance updates for Valkey", function(t)
    t.addHeader("x-user-email", user:email())

	t.query([[
        {
		  team(slug:"devteam") {
		    valkeyInstances {
		      edges {
		        node {
		        	name
		          maintenance {
		            updates {
		              deadline
		              title
		              description
		              documentation_link
		              start_at
		              start_after
		            }
		          }
		        }
		      }
		    }
		  }
		}
	]])

	t.check {
		data = {
			team = {
				valkeyInstances = {
					edges = {},
					nodes = {},
				},
			},
		},
	}
end)
