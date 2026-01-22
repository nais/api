Helper.readK8sResources("k8s_resources/elevation")

local user = User.new("username-1", "user@example.com", "e")
local otherUser = User.new("username-2", "user2@example.com", "e2")

local team = Team.new("myteam", "Elevation test team", "#myteam")
team:addOwner(user)

Test.gql("Create elevation for secret - success", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			createElevation(input: {
				type: SECRET
				team: "myteam"
				environmentName: "dev"
				resourceName: "test-secret"
				reason: "Need to debug database connection issues"
				durationMinutes: 30
			}) {
				elevation {
					id
					type
					team {
						slug
					}
					teamEnvironment {
						name
					}
					resourceName
					user {
						email
					}
					reason
				}
			}
		}
	]]

	t.check {
		data = {
			createElevation = {
				elevation = {
					id = Save("elevationID"),
					type = "SECRET",
					team = {
						slug = "myteam",
					},
					teamEnvironment = {
						name = "dev",
					},
					resourceName = "test-secret",
					user = {
						email = "user@example.com",
					},
					reason = "Need to debug database connection issues",
				},
			},
		},
	}
end)

Test.gql("Query elevations - find created elevation", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			me {
				... on User {
					elevations(input: {
						type: SECRET
						team: "myteam"
						environmentName: "dev"
						resourceName: "test-secret"
					}) {
						id
						type
						resourceName
						reason
					}
				}
			}
		}
	]]

	t.check {
		data = {
			me = {
				elevations = {
					{
						id = State.elevationID,
						type = "SECRET",
						resourceName = "test-secret",
						reason = "Need to debug database connection issues",
					},
				},
			},
		},
	}
end)

Test.gql("Create elevation - reason too short", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			createElevation(input: {
				type: SECRET
				team: "myteam"
				environmentName: "dev"
				resourceName: "test-secret"
				reason: "short"
				durationMinutes: 30
			}) {
				elevation {
					id
				}
			}
		}
	]]

	t.check {
		errors = {
			{
				message = Contains("Reason must be at least 10 characters"),
				path = { "createElevation" },
			},
		},
		data = Null,
	}
end)

Test.gql("Create elevation - duration too long", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			createElevation(input: {
				type: SECRET
				team: "myteam"
				environmentName: "dev"
				resourceName: "test-secret"
				reason: "Need to debug database connection issues"
				durationMinutes: 120
			}) {
				elevation {
					id
				}
			}
		}
	]]

	t.check {
		errors = {
			{
				message = Contains("Duration"),
				path = { "createElevation" },
			},
		},
		data = Null,
	}
end)

Test.gql("Create elevation - non-team member not authorized", function(t)
	t.addHeader("x-user-email", otherUser:email())

	t.query [[
		mutation {
			createElevation(input: {
				type: SECRET
				team: "myteam"
				environmentName: "dev"
				resourceName: "test-secret"
				reason: "Need to debug database connection issues"
				durationMinutes: 30
			}) {
				elevation {
					id
				}
			}
		}
	]]

	t.check {
		errors = {
			{
				message = Contains("authorized"),
				path = { "createElevation" },
			},
		},
		data = Null,
	}
end)

Test.gql("Create elevation - admin user cannot bypass team membership", function(t)
	-- Create an admin user
	local adminUser = User.new("admin-user", "admin@example.com", "admin-ext")
	adminUser:admin(true)

	-- Admin tries to create elevation for myteam (where they are NOT a member)
	t.addHeader("x-user-email", adminUser:email())

	t.query [[
		mutation {
			createElevation(input: {
				type: SECRET
				team: "myteam"
				environmentName: "dev"
				resourceName: "test-secret"
				reason: "Admin trying to access team secrets without membership"
				durationMinutes: 5
			}) {
				elevation {
					id
				}
			}
		}
	]]

	t.check {
		errors = {
			{
				message = Contains("authorized"),
				path = { "createElevation" },
			},
		},
		data = Null,
	}
end)

Test.gql("Create elevation - user from different team cannot elevate", function(t)
	-- Create a second team with otherUser as owner
	local otherTeam = Team.new("otherteam", "Other team", "#otherteam")
	otherTeam:addOwner(otherUser)

	-- otherUser tries to create elevation for myteam's secret (where they are NOT a member)
	t.addHeader("x-user-email", otherUser:email())

	t.query [[
		mutation {
			createElevation(input: {
				type: SECRET
				team: "myteam"
				environmentName: "dev"
				resourceName: "test-secret"
				reason: "Trying to access another team's secrets"
				durationMinutes: 5
			}) {
				elevation {
					id
				}
			}
		}
	]]

	t.check {
		errors = {
			{
				message = Contains("authorized"),
				path = { "createElevation" },
			},
		},
		data = Null,
	}
end)

Test.gql("Create elevation - environment not found", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			createElevation(input: {
				type: SECRET
				team: "myteam"
				environmentName: "nonexistent-env"
				resourceName: "test-secret"
				reason: "Need to debug database connection issues"
				durationMinutes: 30
			}) {
				elevation {
					id
				}
			}
		}
	]]

	t.check {
		errors = {
			{
				message = Contains("Environment"),
				path = { "createElevation" },
			},
		},
		data = Null,
	}
end)

Test.gql("Create elevation for INSTANCE_EXEC", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			createElevation(input: {
				type: INSTANCE_EXEC
				team: "myteam"
				environmentName: "dev"
				resourceName: "test-pod"
				reason: "Need to debug application startup"
				durationMinutes: 15
			}) {
				elevation {
					id
					type
					resourceName
				}
			}
		}
	]]

	t.check {
		data = {
			createElevation = {
				elevation = {
					id = Save("execElevationID"),
					type = "INSTANCE_EXEC",
					resourceName = "test-pod",
				},
			},
		},
	}
end)

Test.gql("Create elevation for INSTANCE_PORT_FORWARD", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			createElevation(input: {
				type: INSTANCE_PORT_FORWARD
				team: "myteam"
				environmentName: "dev"
				resourceName: "test-pod"
				reason: "Need to connect to local database"
				durationMinutes: 15
			}) {
				elevation {
					id
					type
					resourceName
				}
			}
		}
	]]

	t.check {
		data = {
			createElevation = {
				elevation = {
					id = Save("portForwardElevationID"),
					type = "INSTANCE_PORT_FORWARD",
					resourceName = "test-pod",
				},
			},
		},
	}
end)

Test.gql("Create elevation for INSTANCE_DEBUG", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			createElevation(input: {
				type: INSTANCE_DEBUG
				team: "myteam"
				environmentName: "dev"
				resourceName: "test-pod"
				reason: "Need to attach debugger to application"
				durationMinutes: 15
			}) {
				elevation {
					id
					type
					resourceName
				}
			}
		}
	]]

	t.check {
		data = {
			createElevation = {
				elevation = {
					id = Save("debugElevationID"),
					type = "INSTANCE_DEBUG",
					resourceName = "test-pod",
				},
			},
		},
	}
end)

Test.gql("Query elevations - empty when no match", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			me {
				... on User {
					elevations(input: {
						type: SECRET
						team: "myteam"
						environmentName: "dev"
						resourceName: "nonexistent-secret"
					}) {
						id
					}
				}
			}
		}
	]]

	t.check {
		data = {
			me = {
				elevations = {},
			},
		},
	}
end)

Test.gql("Query elevations - other user sees empty list", function(t)
	t.addHeader("x-user-email", otherUser:email())

	t.query [[
		query {
			me {
				... on User {
					elevations(input: {
						type: SECRET
						team: "myteam"
						environmentName: "dev"
						resourceName: "test-secret"
					}) {
						id
					}
				}
			}
		}
	]]

	t.check {
		data = {
			me = {
				elevations = {},
			},
		},
	}
end)
