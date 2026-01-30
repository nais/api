Helper.readK8sResources("k8s_resources/secrets")

local user = User.new("username-1", "user@example.com", "e")
local otherUser = User.new("username-2", "user2@example.com", "e2")

local team = Team.new("myteam", "some purpose", "#channel")
team:addOwner(user)

Test.gql("Create secret for team that does not exist", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			createSecret(input: {
				name: "secret-name"
				environment: "dev"
				team: "does-not-exist"
			}) {
				secret {
					id
				}
			}
		}
	]]

	t.check {
		errors = {
			{
				message = Contains("You are authenticated"),
				path = {
					"createSecret",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Create secret that already exists", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			createSecret(input: {
				name: "unmanaged-secret-in-dev"
				environment: "dev"
				team: "myteam"
			}) {
				secret {
					name
				}
			}
		}
	]]

	t.check {
		errors = {
			{
				message = "A secret with this name already exists.",
				path = {
					"createSecret",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Create secret", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			createSecret(input: {
				name: "secret-name"
				environment: "dev"
				team: "myteam"
			}) {
				secret {
					name
					keys
				}
			}
		}
	]]

	t.check {
		data = {
			createSecret = {
				secret = {
					name = "secret-name",
					keys = {},
				},
			},
		},
	}
end)

Test.gql("Add secret value", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			addSecretValue(input: {
				name: "secret-name"
				environment: "dev"
				team: "myteam"
				value: {
					name: "value-name",
					value: "value"
				}
			}) {
				secret {
					name
					keys
				}
			}
		}
	]]

	t.check {
		data = {
			addSecretValue = {
				secret = {
					name = "secret-name",
					keys = { "value-name" },
				},
			},
		},
	}
end)

Test.gql("Add secret value that already exists", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			addSecretValue(input: {
				name: "secret-name"
				environment: "dev"
				team: "myteam"
				value: {
					name: "value-name",
					value: "value"
				}
			}) {
				secret {
					name
				}
			}
		}
	]]

	t.check {
		errors = {
			{
				message = "The secret already contains a secret value with the name \"value-name\".",
				path = {
					"addSecretValue",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Update secret value", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			updateSecretValue(input: {
				name: "secret-name"
				environment: "dev"
				team: "myteam"
				value: {
					name: "value-name",
					value: "new value"
				}
			}) {
				secret {
					name
					keys
				}
			}
		}
	]]

	t.check {
		data = {
			updateSecretValue = {
				secret = {
					name = "secret-name",
					keys = { "value-name" },
				},
			},
		},
	}
end)

Test.gql("Update secret value that does not exist", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			updateSecretValue(input: {
				name: "secret-name"
				environment: "dev"
				team: "myteam"
				value: {
					name: "does-not-exist",
					value: "new value"
				}
			}) {
				secret {
					name
				}
			}
		}
	]]

	t.check {
		errors = {
			{
				message = "The secret does not contain a secret value with the name \"does-not-exist\".",
				path = {
					"updateSecretValue",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Remove secret value that does not exist", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			removeSecretValue(input: {
				secretName: "secret-name"
				environment: "dev"
				team: "myteam"
				valueName: "foobar"
			}) {
				secret {
					name
				}
			}
		}
	]]

	t.check {
		data = Null,
		errors = {
			{
				message = "The secret does not contain a secret value with the name: \"foobar\".",
				path = {
					"removeSecretValue",
				},
			},
		},
	}
end)

Test.gql("Remove secret value that already exists", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			addSecretValue(input: {
				name: "secret-name"
				environment: "dev"
				team: "myteam"
				value: {
					name: "dont-remove",
					value: "secret"
				}
			}) {
				secret {
					name
					keys
				}
			}
		}
	]]

	t.check {
		data = {
			addSecretValue = {
				secret = {
					name = "secret-name",
					keys = { "dont-remove", "value-name" },
				},
			},
		},
	}

	t.query [[
		mutation {
			removeSecretValue(input: {
				secretName: "secret-name"
				environment: "dev"
				team: "myteam"
				valueName: "value-name"
			}) {
				secret {
					name
					keys
				}
			}
		}
	]]

	t.check {
		data = {
			removeSecretValue = {
				secret = {
					name = "secret-name",
					keys = { "dont-remove" },
				},
			},
		},
	}
end)

Test.gql("Delete secret that does not exist", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			deleteSecret(input: {
				name: "secret-name-that-does-not-exist"
				environment: "dev"
				team: "myteam"
			}) {
				secretDeleted
			}
		}
	]]

	t.check {
		data = Null,
		errors = {
			{
				message = Contains("Resource not found"),
				path = {
					"deleteSecret",
				},
			},
		},
	}
end)

Test.gql("Delete secret that exists", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			deleteSecret(input: {
				name: "secret-name"
				environment: "dev"
				team: "myteam"
			}) {
				secretDeleted
			}
		}
	]]

	t.check {
		data = {
			deleteSecret = {
				secretDeleted = true,
			},
		},
	}
end)

local nonTeamMemberEmail = "email-12@example.com"

Test.gql("Create secret as non-team member", function(t)
	t.addHeader("x-user-email", otherUser:email())

	t.query([[
		mutation {
			createSecret(input: {
				name: "secret-name"
				environment: "dev"
				team: "myteam"
			}) {
				secret {
					name
				}
			}
		}
	]])

	t.check {
		errors = {
			{
				message = Contains("You are authenticated"),
				path = {
					"createSecret",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Update secret as non-team member", function(t)
	t.addHeader("x-user-email", otherUser:email())

	t.query([[
		mutation {
			updateSecretValue(input: {
				name: "secret-name"
				environment: "dev"
				team: "myteam"
				value: {
					name: "value-name",
					value: "new value"
				}
			}) {
				secret {
					name
				}
			}
		}
	]])

	t.check {
		errors = {
			{
				message = Contains("You are authenticated"),
				path = {
					"updateSecretValue",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Delete secret as non-team member", function(t)
	t.addHeader("x-user-email", otherUser:email())

	t.query([[
		mutation {
			deleteSecret(input: {
				name: "secret-name"
				environment: "dev"
				team: "myteam"
			}) {
				secretDeleted
			}
		}
	]])

	t.check {
		errors = {
			{
				message = Contains("You are authenticated"),
				path = {
					"deleteSecret",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Create secret for elevation test", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			createSecret(input: {
				name: "test-elevation-secret"
				environment: "dev"
				team: "myteam"
			}) {
				secret {
					name
				}
			}
		}
	]]

	t.check {
		data = {
			createSecret = {
				secret = {
					name = "test-elevation-secret",
				},
			},
		},
	}

	-- Add a value
	t.query [[
		mutation {
			addSecretValue(input: {
				name: "test-elevation-secret"
				environment: "dev"
				team: "myteam"
				value: {
					name: "api-key",
					value: "super-secret-123"
				}
			}) {
				secret {
					name
					keys
				}
			}
		}
	]]

	t.check {
		data = {
			addSecretValue = {
				secret = {
					name = "test-elevation-secret",
					keys = { "api-key" },
				},
			},
		},
	}
end)

Test.gql("Reading secret values WITHOUT elevation should fail", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "myteam") {
				environment(name: "dev") {
					secret(name: "test-elevation-secret") {
						name
						values {
							name
							value
						}
					}
				}
			}
		}
	]]

	t.check {
		errors = {
			{
				message = Contains("You are authenticated"),
				path = { "team", "environment", "secret", "values" },
			},
		},
		data = {
			team = {
				environment = {
					secret = {
						name = "test-elevation-secret",
						values = Null,
					},
				},
			},
		},
	}
end)

Test.gql("Create elevation for reading secret values", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			createElevation(input: {
				type: SECRET
				team: "myteam"
				environmentName: "dev"
				resourceName: "test-elevation-secret"
				reason: "Testing secret values access with elevation"
				durationMinutes: 5
			}) {
				elevation {
					id
				}
			}
		}
	]]

	t.check {
		data = {
			createElevation = {
				elevation = {
					id = Save("elevationID"),
				},
			},
		},
	}
end)

Test.gql("Reading secret values WITH elevation should succeed", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "myteam") {
				environment(name: "dev") {
					secret(name: "test-elevation-secret") {
						name
						values {
							name
							value
						}
					}
				}
			}
		}
	]]

	t.check {
		data = {
			team = {
				environment = {
					secret = {
						name = "test-elevation-secret",
						values = {
							{
								name = "api-key",
								value = "super-secret-123",
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("Admin user cannot read secret values without team membership", function(t)
	-- Create an admin user (not a member of myteam)
	local adminUser = User.new("admin-user", "admin@example.com", "admin-ext")
	adminUser:admin(true)

	-- Admin tries to read secret values without being a team member
	-- Since admin is not a team member, they cannot read secret values even with admin privileges
	t.addHeader("x-user-email", adminUser:email())

	t.query [[
		query {
			team(slug: "myteam") {
				environment(name: "dev") {
					secret(name: "test-elevation-secret") {
						name
						values {
							name
							value
						}
					}
				}
			}
		}
	]]

	-- Admin can see secret metadata but cannot read values without team membership
	t.check {
		errors = {
			{
				message = Contains("You are authenticated"),
				path = { "team", "environment", "secret", "values" },
			},
		},
		data = {
			team = {
				environment = {
					secret = {
						name = "test-elevation-secret",
						values = Null,
					},
				},
			},
		},
	}
end)


Test.gql("Admin can delete secret in other team", function(t)
	-- Use unique admin user for this test
	local adminUser = User.new("admin-delete-test", "admin-delete@example.com", "admin-del")
	adminUser:admin(true)

	local teamOwner = User.new("team-owner-del", "owner-del@example.com", "owner-del")
	local otherTeam = Team.new("admindeltest", "admin delete test team", "#channel")
	otherTeam:addOwner(teamOwner)

	-- Create a secret in the team as team owner
	t.addHeader("x-user-email", teamOwner:email())
	t.query [[
		mutation {
			createSecret(input: {
				name: "admin-delete-test"
				environment: "dev"
				team: "admindeltest"
			}) {
				secret {
					name
				}
			}
		}
	]]

	t.check {
		data = {
			createSecret = {
				secret = {
					name = "admin-delete-test",
				},
			},
		},
	}

	-- Admin (not team member) should be able to delete it
	t.addHeader("x-user-email", adminUser:email())
	t.query [[
		mutation {
			deleteSecret(input: {
				name: "admin-delete-test"
				environment: "dev"
				team: "admindeltest"
			}) {
				secretDeleted
			}
		}
	]]

	t.check {
		data = {
			deleteSecret = {
				secretDeleted = true,
			},
		},
	}
end)

Test.gql("Admin can manage secrets but CANNOT read values without team membership", function(t)
	-- Use unique admin user for this test
	local adminUser = User.new("admin-readonly-test", "admin-readonly@example.com", "admin-ro")
	adminUser:admin(true)

	local otherTeam = Team.new("adminrotest", "admin readonly test team", "#channel")

	-- Admin should be able to create secret (metadata operation)
	t.addHeader("x-user-email", adminUser:email())
	t.query [[
		mutation {
			createSecret(input: {
				name: "admin-managed-secret"
				environment: "dev"
				team: "adminrotest"
			}) {
				secret {
					name
				}
			}
		}
	]]

	t.check {
		data = {
			createSecret = {
				secret = {
					name = "admin-managed-secret",
				},
			},
		},
	}

	-- Admin should be able to add secret value (using JSON Patch - doesn't read values)
	t.query [[
		mutation {
			addSecretValue(input: {
				name: "admin-managed-secret"
				environment: "dev"
				team: "adminrotest"
				value: {
					name: "API_KEY"
					value: "secret-value"
				}
			}) {
				secret {
					name
					keys
				}
			}
		}
	]]

	t.check {
		data = {
			addSecretValue = {
				secret = {
					name = "admin-managed-secret",
					keys = { "API_KEY" },
				},
			},
		},
	}

	-- Admin should NOT be able to read secret values (requires team membership + elevation)
	local secret = t.query [[
		query {
			team(slug: "adminrotest") {
				environment(name: "dev") {
					secret(name: "admin-managed-secret") {
						name
						values {
							name
							value
						}
					}
				}
			}
		}
	]]

	t.check {
		errors = {
			{
				message = Contains("You are authenticated"),
				path = { "team", "environment", "secret", "values" },
			},
		},
		data = {
			team = {
				environment = {
					secret = {
						name = "admin-managed-secret",
						values = Null,
					},
				},
			},
		},
	}
end)

-- Tests for viewSecretValues mutation

Test.gql("viewSecretValues - success with valid reason", function(t)
	t.addHeader("x-user-email", user:email())

	-- First create a secret with a value
	t.query [[
		mutation {
			createSecret(input: {
				name: "view-test-secret"
				environment: "dev"
				team: "myteam"
			}) {
				secret { name }
			}
		}
	]]

	t.check {
		data = {
			createSecret = {
				secret = { name = "view-test-secret" },
			},
		},
	}

	t.query [[
		mutation {
			addSecretValue(input: {
				name: "view-test-secret"
				environment: "dev"
				team: "myteam"
				value: {
					name: "DATABASE_URL",
					value: "postgres://localhost/mydb"
				}
			}) {
				secret { name keys }
			}
		}
	]]

	t.check {
		data = {
			addSecretValue = {
				secret = {
					name = "view-test-secret",
					keys = { "DATABASE_URL" },
				},
			},
		},
	}

	-- First verify that direct read via values resolver fails (requires elevation)
	t.query [[
		query {
			team(slug: "myteam") {
				environment(name: "dev") {
					secret(name: "view-test-secret") {
						name
						values {
							name
							value
						}
					}
				}
			}
		}
	]]

	t.check {
		errors = {
			{
				message = Contains("You are authenticated"),
				path = { "team", "environment", "secret", "values" },
			},
		},
		data = {
			team = {
				environment = {
					secret = {
						name = "view-test-secret",
						values = Null,
					},
				},
			},
		},
	}

	-- Now use viewSecretValues to read the values (should succeed without separate elevation)
	t.query [[
		mutation {
			viewSecretValues(input: {
				name: "view-test-secret"
				environment: "dev"
				team: "myteam"
				reason: "Testing viewSecretValues mutation for database migration"
			}) {
				values {
					name
					value
				}
			}
		}
	]]

	t.check {
		data = {
			viewSecretValues = {
				values = {
					{
						name = "DATABASE_URL",
						value = "postgres://localhost/mydb",
					},
				},
			},
		},
	}
end)

Test.gql("viewSecretValues - fails with reason too short", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			viewSecretValues(input: {
				name: "view-test-secret"
				environment: "dev"
				team: "myteam"
				reason: "too short"
			}) {
				values {
					name
					value
				}
			}
		}
	]]

	t.check {
		errors = {
			{
				message = Contains("at least 10 characters"),
				path = { "viewSecretValues" },
			},
		},
		data = Null,
	}
end)

Test.gql("viewSecretValues - fails for non-team member", function(t)
	t.addHeader("x-user-email", otherUser:email())

	t.query [[
		mutation {
			viewSecretValues(input: {
				name: "view-test-secret"
				environment: "dev"
				team: "myteam"
				reason: "I want to see the secret values for debugging"
			}) {
				values {
					name
					value
				}
			}
		}
	]]

	t.check {
		errors = {
			{
				message = Contains("You are authenticated"),
				path = { "viewSecretValues" },
			},
		},
		data = Null,
	}
end)

Test.gql("viewSecretValues - admin cannot bypass team membership", function(t)
	local adminUser = User.new("admin-view-test", "admin-view@example.com", "admin-view")
	adminUser:admin(true)

	t.addHeader("x-user-email", adminUser:email())

	t.query [[
		mutation {
			viewSecretValues(input: {
				name: "view-test-secret"
				environment: "dev"
				team: "myteam"
				reason: "Admin trying to view secret without team membership"
			}) {
				values {
					name
					value
				}
			}
		}
	]]

	t.check {
		errors = {
			{
				message = Contains("You are authenticated"),
				path = { "viewSecretValues" },
			},
		},
		data = Null,
	}
end)

Test.gql("viewSecretValues - logs access to activity log", function(t)
	t.addHeader("x-user-email", user:email())

	-- First create a secret with a value (each test runs in isolation when filtered)
	t.query [[
		mutation {
			createSecret(input: {
				name: "activity-log-test-secret"
				environment: "dev"
				team: "myteam"
			}) {
				secret { name }
			}
		}
	]]

	t.check {
		data = {
			createSecret = {
				secret = { name = "activity-log-test-secret" },
			},
		},
	}

	t.query [[
		mutation {
			addSecretValue(input: {
				name: "activity-log-test-secret"
				environment: "dev"
				team: "myteam"
				value: {
					name: "TEST_KEY",
					value: "test-value"
				}
			}) {
				secret { name keys }
			}
		}
	]]

	t.check {
		data = {
			addSecretValue = {
				secret = {
					name = "activity-log-test-secret",
					keys = { "TEST_KEY" },
				},
			},
		},
	}

	-- View the secret values (this should create an activity log entry)
	t.query [[
		mutation {
			viewSecretValues(input: {
				name: "activity-log-test-secret"
				environment: "dev"
				team: "myteam"
				reason: "Checking activity log for secret access audit"
			}) {
				values {
					name
					value
				}
			}
		}
	]]

	t.check {
		data = {
			viewSecretValues = {
				values = {
					{
						name = "TEST_KEY",
						value = "test-value",
					},
				},
			},
		},
	}

	-- Check that the activity log entry was created (use first:1 to get most recent)
	t.query [[
		query {
			team(slug: "myteam") {
				activityLog(first: 1, filter: { activityTypes: [SECRET_VALUES_VIEWED] }) {
					nodes {
						... on SecretValuesViewedActivityLogEntry {
							message
							resourceName
							data {
								reason
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
				activityLog = {
					nodes = {
						{
							message = "Viewed secret values",
							resourceName = "activity-log-test-secret",
							data = {
								reason = "Checking activity log for secret access audit",
							},
						},
					},
				},
			},
		},
	}
end)
