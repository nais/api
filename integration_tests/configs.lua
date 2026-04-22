Helper.readK8sResources("k8s_resources/configs")

local user = User.new("username-1", "user@example.com", "e")
local otherUser = User.new("username-2", "user2@example.com", "e2")

local team = Team.new("myteam", "some purpose", "#channel")
team:addOwner(user)

Test.gql("Create config for team that does not exist", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			createConfig(input: {
				name: "config-name"
				environmentName: "dev"
				teamSlug: "does-not-exist"
			}) {
				config {
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
					"createConfig",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Create config that already exists", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			createConfig(input: {
				name: "managed-config-in-dev"
				environmentName: "dev"
				teamSlug: "myteam"
			}) {
				config {
					name
				}
			}
		}
	]]

	t.check {
		errors = {
			{
				message = "A config with this name already exists.",
				path = {
					"createConfig",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Create config", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			createConfig(input: {
				name: "config-name"
				environmentName: "dev"
				teamSlug: "myteam"
			}) {
				config {
					name
					values {
						name
						value
					}
				}
			}
		}
	]]

	t.check {
		data = {
			createConfig = {
				config = {
					name = "config-name",
					values = {},
				},
			},
		},
	}
end)

Test.gql("Add config value", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			addConfigValue(input: {
				name: "config-name"
				environmentName: "dev"
				teamSlug: "myteam"
				value: {
					name: "value-name",
					value: "value"
				}
			}) {
				config {
					name
					values {
						name
						value
					}
				}
			}
		}
	]]

	t.check {
		data = {
			addConfigValue = {
				config = {
					name = "config-name",
					values = {
						{
							name = "value-name",
							value = "value",
						},
					},
				},
			},
		},
	}
end)

Test.gql("Add config value that already exists", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			addConfigValue(input: {
				name: "config-name"
				environmentName: "dev"
				teamSlug: "myteam"
				value: {
					name: "value-name",
					value: "value"
				}
			}) {
				config {
					name
				}
			}
		}
	]]

	t.check {
		errors = {
			{
				message = Contains("already contains a value with the name"),
				path = {
					"addConfigValue",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Update config value", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			updateConfigValue(input: {
				name: "config-name"
				environmentName: "dev"
				teamSlug: "myteam"
				value: {
					name: "value-name",
					value: "new value"
				}
			}) {
				config {
					name
					values {
						name
						value
					}
				}
			}
		}
	]]

	t.check {
		data = {
			updateConfigValue = {
				config = {
					name = "config-name",
					values = {
						{
							name = "value-name",
							value = "new value",
						},
					},
				},
			},
		},
	}
end)

Test.gql("Update config value that does not exist", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			updateConfigValue(input: {
				name: "config-name"
				environmentName: "dev"
				teamSlug: "myteam"
				value: {
					name: "does-not-exist",
					value: "new value"
				}
			}) {
				config {
					name
				}
			}
		}
	]]

	t.check {
		errors = {
			{
				message = Contains("does not contain a value with the name"),
				path = {
					"updateConfigValue",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Remove config value that does not exist", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			removeConfigValue(input: {
				configName: "config-name"
				environmentName: "dev"
				teamSlug: "myteam"
				valueName: "foobar"
			}) {
				config {
					name
				}
			}
		}
	]]

	t.check {
		data = Null,
		errors = {
			{
				message = Contains("does not contain a value with the name"),
				path = {
					"removeConfigValue",
				},
			},
		},
	}
end)

Test.gql("Remove config value that exists", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			addConfigValue(input: {
				name: "config-name"
				environmentName: "dev"
				teamSlug: "myteam"
				value: {
					name: "dont-remove",
					value: "keep-this"
				}
			}) {
				config {
					name
					values {
						name
						value
					}
				}
			}
		}
	]]

	t.check {
		data = {
			addConfigValue = {
				config = {
					name = "config-name",
					values = {
						{
							name = "dont-remove",
							value = "keep-this",
						},
						{
							name = "value-name",
							value = "new value",
						},
					},
				},
			},
		},
	}

	t.query [[
		mutation {
			removeConfigValue(input: {
				configName: "config-name"
				environmentName: "dev"
				teamSlug: "myteam"
				valueName: "value-name"
			}) {
				config {
					name
					values {
						name
						value
					}
				}
			}
		}
	]]

	t.check {
		data = {
			removeConfigValue = {
				config = {
					name = "config-name",
					values = {
						{
							name = "dont-remove",
							value = "keep-this",
						},
					},
				},
			},
		},
	}
end)

Test.gql("Read config values directly", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		{
			team(slug: "myteam") {
				environment(name: "dev") {
					config(name: "config-name") {
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
					config = {
						name = "config-name",
						values = {
							{
								name = "dont-remove",
								value = "keep-this",
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("Delete config that does not exist", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			deleteConfig(input: {
				name: "config-name-that-does-not-exist"
				environmentName: "dev"
				teamSlug: "myteam"
			}) {
				configDeleted
			}
		}
	]]

	t.check {
		data = Null,
		errors = {
			{
				message = Contains("Resource not found"),
				path = {
					"deleteConfig",
				},
			},
		},
	}
end)

Test.gql("Delete config that exists", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			deleteConfig(input: {
				name: "config-name"
				environmentName: "dev"
				teamSlug: "myteam"
			}) {
				configDeleted
			}
		}
	]]

	t.check {
		data = {
			deleteConfig = {
				configDeleted = true,
			},
		},
	}
end)

-- Authorization tests

Test.gql("Create config as non-team member", function(t)
	t.addHeader("x-user-email", otherUser:email())

	t.query([[
		mutation {
			createConfig(input: {
				name: "config-name"
				environmentName: "dev"
				teamSlug: "myteam"
			}) {
				config {
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
					"createConfig",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Update config as non-team member", function(t)
	t.addHeader("x-user-email", otherUser:email())

	t.query([[
		mutation {
			updateConfigValue(input: {
				name: "managed-config-in-dev"
				environmentName: "dev"
				teamSlug: "myteam"
				value: {
					name: "username",
					value: "new value"
				}
			}) {
				config {
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
					"updateConfigValue",
				},
			},
		},
		data = Null,
	}
end)

Test.gql("Delete config as non-team member", function(t)
	t.addHeader("x-user-email", otherUser:email())

	t.query([[
		mutation {
			deleteConfig(input: {
				name: "managed-config-in-dev"
				environmentName: "dev"
				teamSlug: "myteam"
			}) {
				configDeleted
			}
		}
	]])

	t.check {
		errors = {
			{
				message = Contains("You are authenticated"),
				path = {
					"deleteConfig",
				},
			},
		},
		data = Null,
	}
end)

-- Admin tests

Test.gql("Admin can delete config in other team", function(t)
	local adminUser = User.new("admin-delete-test", "admin-delete@example.com", "admin-del")
	adminUser:admin(true)

	local teamOwner = User.new("team-owner-del", "owner-del@example.com", "owner-del")
	local otherTeam = Team.new("admindeltest", "admin delete test team", "#channel")
	otherTeam:addOwner(teamOwner)

	-- Create a config in the team as team owner
	t.addHeader("x-user-email", teamOwner:email())
	t.query [[
		mutation {
			createConfig(input: {
				name: "admin-delete-test"
				environmentName: "dev"
				teamSlug: "admindeltest"
			}) {
				config {
					name
				}
			}
		}
	]]

	t.check {
		data = {
			createConfig = {
				config = {
					name = "admin-delete-test",
				},
			},
		},
	}

	-- Admin (not team member) should be able to delete it
	t.addHeader("x-user-email", adminUser:email())
	t.query [[
		mutation {
			deleteConfig(input: {
				name: "admin-delete-test"
				environmentName: "dev"
				teamSlug: "admindeltest"
			}) {
				configDeleted
			}
		}
	]]

	t.check {
		data = {
			deleteConfig = {
				configDeleted = true,
			},
		},
	}
end)

Test.gql("Admin can manage configs in other team", function(t)
	local adminUser = User.new("admin-manage-test", "admin-manage@example.com", "admin-mgmt")
	adminUser:admin(true)

	local otherTeam = Team.new("adminmgmttest", "admin manage test team", "#channel")

	-- Admin should be able to create config (metadata operation)
	t.addHeader("x-user-email", adminUser:email())
	t.query [[
		mutation {
			createConfig(input: {
				name: "admin-managed-config"
				environmentName: "dev"
				teamSlug: "adminmgmttest"
			}) {
				config {
					name
				}
			}
		}
	]]

	t.check {
		data = {
			createConfig = {
				config = {
					name = "admin-managed-config",
				},
			},
		},
	}

	-- Admin should be able to add config value
	t.query [[
		mutation {
			addConfigValue(input: {
				name: "admin-managed-config"
				environmentName: "dev"
				teamSlug: "adminmgmttest"
				value: {
					name: "API_URL"
					value: "https://example.com"
				}
			}) {
				config {
					name
					values {
						name
						value
					}
				}
			}
		}
	]]

	t.check {
		data = {
			addConfigValue = {
				config = {
					name = "admin-managed-config",
					values = {
						{
							name = "API_URL",
							value = "https://example.com",
						},
					},
				},
			},
		},
	}
end)

-- Test listing configs (before activity log test to avoid leaked state)

Test.gql("List team configs", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		{
			team(slug: "myteam") {
				configs {
					nodes {
						name
					}
				}
			}
		}
	]]

	t.check {
		data = {
			team = {
				configs = {
					nodes = {
						{
							name = "managed-config-in-dev",
						},
						{
							name = "managed-config-in-staging",
						},
						{
							name = "managed-config-in-staging-used-with-filesfrom",
						},
					},
				},
			},
		},
	}
end)

Test.gql("List configs with values visible", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		{
			team(slug: "myteam") {
				environment(name: "dev") {
					config(name: "managed-config-in-dev") {
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
					config = {
						name = "managed-config-in-dev",
						values = {
							{
								name = "password",
								value = "hunter2",
							},
							{
								name = "username",
								value = "admin",
							},
						},
					},
				},
			},
		},
	}
end)

-- Test that unmanaged configs are not visible

Test.gql("Unmanaged config not in list", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		{
			team(slug: "myteam") {
				environment(name: "dev") {
					config(name: "unmanaged-config-in-dev") {
						name
					}
				}
			}
		}
	]]

	t.check {
		data = Null,
		errors = {
			{
				message = Contains("not found"),
				path = {
					"team",
					"environment",
					"config",
				},
			},
		},
	}
end)

-- Test that values like "true" are not misdetected as binary

Test.gql("Config value true is returned as plain text", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			createConfig(input: {
				name: "true-value-test"
				environmentName: "dev"
				teamSlug: "myteam"
			}) {
				config { name }
			}
		}
	]]

	t.check {
		data = {
			createConfig = {
				config = { name = "true-value-test" },
			},
		},
	}

	t.query [[
		mutation {
			addConfigValue(input: {
				name: "true-value-test"
				environmentName: "dev"
				teamSlug: "myteam"
				value: {
					name: "MY_FLAG",
					value: "true"
				}
			}) {
				config {
					name
					values {
						name
						value
						encoding
					}
				}
			}
		}
	]]

	t.check {
		data = {
			addConfigValue = {
				config = {
					name = "true-value-test",
					values = {
						{
							name = "MY_FLAG",
							value = "true",
							encoding = "PLAIN_TEXT",
						},
					},
				},
			},
		},
	}

	-- Verify the value is also correct when read back via query
	t.query [[
		{
			team(slug: "myteam") {
				environment(name: "dev") {
					config(name: "true-value-test") {
						values {
							name
							value
							encoding
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
					config = {
						values = {
							{
								name = "MY_FLAG",
								value = "true",
								encoding = "PLAIN_TEXT",
							},
						},
					},
				},
			},
		},
	}

	-- Clean up
	t.query [[
		mutation {
			deleteConfig(input: {
				name: "true-value-test"
				environmentName: "dev"
				teamSlug: "myteam"
			}) {
				configDeleted
			}
		}
	]]

	t.check {
		data = {
			deleteConfig = {
				configDeleted = true,
			},
		},
	}
end)

-- Test binary data in configs using binaryData field

Test.gql("Config binary data round-trip", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			createConfig(input: {
				name: "binary-data-test"
				environmentName: "dev"
				teamSlug: "myteam"
			}) {
				config { name }
			}
		}
	]]

	t.check {
		data = {
			createConfig = {
				config = { name = "binary-data-test" },
			},
		},
	}

	-- Add a binary value (BASE64 encoded)
	t.query [[
		mutation {
			addConfigValue(input: {
				name: "binary-data-test"
				environmentName: "dev"
				teamSlug: "myteam"
				value: {
					name: "tls.crt",
					value: "AAEC/w==",
					encoding: BASE64
				}
			}) {
				config {
					name
					values {
						name
						value
						encoding
					}
				}
			}
		}
	]]

	t.check {
		data = {
			addConfigValue = {
				config = {
					name = "binary-data-test",
					values = {
						{
							name = "tls.crt",
							value = "AAEC/w==",
							encoding = "BASE64",
						},
					},
				},
			},
		},
	}

	-- Add a plain text value alongside the binary one
	t.query [[
		mutation {
			addConfigValue(input: {
				name: "binary-data-test"
				environmentName: "dev"
				teamSlug: "myteam"
				value: {
					name: "log-level",
					value: "info"
				}
			}) {
				config {
					name
					values {
						name
						value
						encoding
					}
				}
			}
		}
	]]

	t.check {
		data = {
			addConfigValue = {
				config = {
					name = "binary-data-test",
					values = {
						{
							name = "log-level",
							value = "info",
							encoding = "PLAIN_TEXT",
						},
						{
							name = "tls.crt",
							value = "AAEC/w==",
							encoding = "BASE64",
						},
					},
				},
			},
		},
	}

	-- Read back via query to verify round-trip
	t.query [[
		{
			team(slug: "myteam") {
				environment(name: "dev") {
					config(name: "binary-data-test") {
						values {
							name
							value
							encoding
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
					config = {
						values = {
							{
								name = "log-level",
								value = "info",
								encoding = "PLAIN_TEXT",
							},
							{
								name = "tls.crt",
								value = "AAEC/w==",
								encoding = "BASE64",
							},
						},
					},
				},
			},
		},
	}

	-- Update binary value to plain text (should move from binaryData to data)
	t.query [[
		mutation {
			updateConfigValue(input: {
				name: "binary-data-test"
				environmentName: "dev"
				teamSlug: "myteam"
				value: {
					name: "tls.crt",
					value: "now-plain-text"
				}
			}) {
				config {
					values {
						name
						value
						encoding
					}
				}
			}
		}
	]]

	t.check {
		data = {
			updateConfigValue = {
				config = {
					values = {
						{
							name = "log-level",
							value = "info",
							encoding = "PLAIN_TEXT",
						},
						{
							name = "tls.crt",
							value = "now-plain-text",
							encoding = "PLAIN_TEXT",
						},
					},
				},
			},
		},
	}

	-- Update plain text value to binary (should move from data to binaryData)
	t.query [[
		mutation {
			updateConfigValue(input: {
				name: "binary-data-test"
				environmentName: "dev"
				teamSlug: "myteam"
				value: {
					name: "tls.crt",
					value: "AAEC/w==",
					encoding: BASE64
				}
			}) {
				config {
					values {
						name
						value
						encoding
					}
				}
			}
		}
	]]

	t.check {
		data = {
			updateConfigValue = {
				config = {
					values = {
						{
							name = "log-level",
							value = "info",
							encoding = "PLAIN_TEXT",
						},
						{
							name = "tls.crt",
							value = "AAEC/w==",
							encoding = "BASE64",
						},
					},
				},
			},
		},
	}

	-- Remove the binary value
	t.query [[
		mutation {
			removeConfigValue(input: {
				configName: "binary-data-test"
				environmentName: "dev"
				teamSlug: "myteam"
				valueName: "tls.crt"
			}) {
				config {
					values {
						name
						value
						encoding
					}
				}
			}
		}
	]]

	t.check {
		data = {
			removeConfigValue = {
				config = {
					values = {
						{
							name = "log-level",
							value = "info",
							encoding = "PLAIN_TEXT",
						},
					},
				},
			},
		},
	}

	-- Clean up
	t.query [[
		mutation {
			deleteConfig(input: {
				name: "binary-data-test"
				environmentName: "dev"
				teamSlug: "myteam"
			}) {
				configDeleted
			}
		}
	]]

	t.check {
		data = {
			deleteConfig = {
				configDeleted = true,
			},
		},
	}
end)

-- Activity log tests

Test.gql("Config CRUD creates activity log entries", function(t)
	t.addHeader("x-user-email", user:email())

	-- Create a config
	t.query [[
		mutation {
			createConfig(input: {
				name: "activity-log-test"
				environmentName: "dev"
				teamSlug: "myteam"
			}) {
				config { name }
			}
		}
	]]

	t.check {
		data = {
			createConfig = {
				config = { name = "activity-log-test" },
			},
		},
	}

	-- Add a value
	t.query [[
		mutation {
			addConfigValue(input: {
				name: "activity-log-test"
				environmentName: "dev"
				teamSlug: "myteam"
				value: {
					name: "TEST_KEY",
					value: "test-value"
				}
			}) {
				config { name }
			}
		}
	]]

	t.check {
		data = {
			addConfigValue = {
				config = { name = "activity-log-test" },
			},
		},
	}

	-- Update the value
	t.query [[
		mutation {
			updateConfigValue(input: {
				name: "activity-log-test"
				environmentName: "dev"
				teamSlug: "myteam"
				value: {
					name: "TEST_KEY",
					value: "updated-value"
				}
			}) {
				config { name }
			}
		}
	]]

	t.check {
		data = {
			updateConfigValue = {
				config = { name = "activity-log-test" },
			},
		},
	}

	-- Check activity log for config created entry (use first: 1 to get the most recent)
	t.query [[
		query {
			team(slug: "myteam") {
				activityLog(first: 1, filter: { activityTypes: [CONFIG_CREATED] }) {
					nodes {
						... on ConfigCreatedActivityLogEntry {
							message
							resourceName
							resourceType
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
							message = Contains("Created config"),
							resourceName = "activity-log-test",
							resourceType = "CONFIG",
						},
					},
				},
			},
		},
	}

	-- Check activity log for config updated entries (add + update = 2 entries)
	t.query [[
		query {
			team(slug: "myteam") {
				activityLog(first: 2, filter: { activityTypes: [CONFIG_UPDATED] }) {
					nodes {
						... on ConfigUpdatedActivityLogEntry {
							message
							resourceName
							data {
								updatedFields {
									field
									oldValue
									newValue
								}
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
							message = Contains("Updated config"),
							resourceName = "activity-log-test",
							data = {
								updatedFields = {
									{
										field = "TEST_KEY",
										oldValue = "test-value",
										newValue = "updated-value",
									},
								},
							},
						},
						{
							message = Contains("Updated config"),
							resourceName = "activity-log-test",
							data = {
								updatedFields = {
									{
										field = "TEST_KEY",
										oldValue = Null,
										newValue = "test-value",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	-- Delete the config
	t.query [[
		mutation {
			deleteConfig(input: {
				name: "activity-log-test"
				environmentName: "dev"
				teamSlug: "myteam"
			}) {
				configDeleted
			}
		}
	]]

	t.check {
		data = {
			deleteConfig = {
				configDeleted = true,
			},
		},
	}

	-- Check activity log for config deleted entry
	t.query [[
		query {
			team(slug: "myteam") {
				activityLog(first: 1, filter: { activityTypes: [CONFIG_DELETED] }) {
					nodes {
						... on ConfigDeletedActivityLogEntry {
							message
							resourceName
							resourceType
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
							message = Contains("Deleted config"),
							resourceName = "activity-log-test",
							resourceType = "CONFIG",
						},
					},
				},
			},
		},
	}
end)
