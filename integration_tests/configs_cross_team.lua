-- Test cross-team config access control
-- Verifies that users from other teams:
-- 1. CAN see config names and values (config values are not sensitive)
-- 2. CANNOT create, update, or delete configs

Helper.readK8sResources("k8s_resources/configs")

local teamOwner = User.new("team-owner", "owner@example.com", "ext-owner")
local otherUser = User.new("other-user", "other@example.com", "ext-other")

-- Setup: Create two teams
Test.gql("Create team A (owns configs)", function(t)
	t.addHeader("x-user-email", teamOwner:email())

	t.query [[
		mutation {
			createTeam(
				input: {
					slug: "alpha"
					purpose: "Team that owns configs"
					slackChannel: "#alpha"
				}
			) {
				team {
					slug
				}
			}
		}
	]]

	t.check {
		data = {
			createTeam = {
				team = {
					slug = "alpha",
				},
			},
		},
	}
end)

Test.gql("Create team B (other team)", function(t)
	t.addHeader("x-user-email", otherUser:email())

	t.query [[
		mutation {
			createTeam(
				input: {
					slug: "beta"
					purpose: "Other team"
					slackChannel: "#beta"
				}
			) {
				team {
					slug
				}
			}
		}
	]]

	t.check {
		data = {
			createTeam = {
				team = {
					slug = "beta",
				},
			},
		},
	}
end)

-- Team owner creates a config with values
Test.gql("Team owner creates config with values", function(t)
	t.addHeader("x-user-email", teamOwner:email())

	t.query [[
		mutation {
			createConfig(input: {
				name: "alpha-config"
				environmentName: "dev"
				teamSlug: "alpha"
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
					name = "alpha-config",
				},
			},
		},
	}

	-- Add some values
	t.query [[
		mutation {
			addConfigValue(input: {
				name: "alpha-config"
				environmentName: "dev"
				teamSlug: "alpha"
				value: {
					name: "api-url"
					value: "https://api.example.com"
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
					name = "alpha-config",
					values = {
						{
							name = "api-url",
							value = "https://api.example.com",
						},
					},
				},
			},
		},
	}
end)

-- ============================================================
-- Cross-team READ tests (should be allowed - config values are not sensitive)
-- ============================================================

Test.gql("Other team CAN see config names via team.configs", function(t)
	t.addHeader("x-user-email", otherUser:email())

	t.query [[
		query {
			team(slug: "alpha") {
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
							name = "alpha-config",
						},
					},
				},
			},
		},
	}
end)

Test.gql("Other team CAN see config values", function(t)
	t.addHeader("x-user-email", otherUser:email())

	t.query [[
		query {
			team(slug: "alpha") {
				environment(name: "dev") {
					config(name: "alpha-config") {
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
						name = "alpha-config",
						values = {
							{
								name = "api-url",
								value = "https://api.example.com",
							},
						},
					},
				},
			},
		},
	}
end)

-- ============================================================
-- Cross-team MUTATION tests (should all be BLOCKED)
-- ============================================================

Test.gql("Other team CANNOT create config in alpha", function(t)
	t.addHeader("x-user-email", otherUser:email())

	t.query [[
		mutation {
			createConfig(input: {
				name: "malicious-config"
				environmentName: "dev"
				teamSlug: "alpha"
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
				message = Contains("You are authenticated"),
				path = { "createConfig" },
			},
		},
		data = Null,
	}
end)

Test.gql("Other team CANNOT add value to alpha config", function(t)
	t.addHeader("x-user-email", otherUser:email())

	t.query [[
		mutation {
			addConfigValue(input: {
				name: "alpha-config"
				environmentName: "dev"
				teamSlug: "alpha"
				value: {
					name: "malicious-key"
					value: "malicious-value"
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
				message = Contains("You are authenticated"),
				path = { "addConfigValue" },
			},
		},
		data = Null,
	}
end)

Test.gql("Other team CANNOT update value in alpha config", function(t)
	t.addHeader("x-user-email", otherUser:email())

	t.query [[
		mutation {
			updateConfigValue(input: {
				name: "alpha-config"
				environmentName: "dev"
				teamSlug: "alpha"
				value: {
					name: "api-url"
					value: "https://hacked.example.com"
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
				message = Contains("You are authenticated"),
				path = { "updateConfigValue" },
			},
		},
		data = Null,
	}
end)

Test.gql("Other team CANNOT remove value from alpha config", function(t)
	t.addHeader("x-user-email", otherUser:email())

	t.query [[
		mutation {
			removeConfigValue(input: {
				configName: "alpha-config"
				environmentName: "dev"
				teamSlug: "alpha"
				valueName: "api-url"
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
				message = Contains("You are authenticated"),
				path = { "removeConfigValue" },
			},
		},
		data = Null,
	}
end)

Test.gql("Other team CANNOT delete alpha config", function(t)
	t.addHeader("x-user-email", otherUser:email())

	t.query [[
		mutation {
			deleteConfig(input: {
				name: "alpha-config"
				environmentName: "dev"
				teamSlug: "alpha"
			}) {
				configDeleted
			}
		}
	]]

	t.check {
		errors = {
			{
				message = Contains("You are authenticated"),
				path = { "deleteConfig" },
			},
		},
		data = Null,
	}
end)

-- ============================================================
-- Verify team owner still has full access
-- ============================================================

Test.gql("Team owner CAN delete config (cleanup)", function(t)
	t.addHeader("x-user-email", teamOwner:email())

	t.query [[
		mutation {
			deleteConfig(input: {
				name: "alpha-config"
				environmentName: "dev"
				teamSlug: "alpha"
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
