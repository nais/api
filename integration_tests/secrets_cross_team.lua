-- Test cross-team secret access control
-- Verifies that users from other teams:
-- 1. CAN see secret names and keys (metadata)
-- 2. CANNOT see secret values
-- 3. CANNOT create, update, or delete secrets

Helper.readK8sResources("k8s_resources/secrets")

local teamOwner = User.new("team-owner", "owner@example.com", "ext-owner")
local otherUser = User.new("other-user", "other@example.com", "ext-other")

-- Setup: Create two teams
Test.gql("Create team A (owns secrets)", function(t)
	t.addHeader("x-user-email", teamOwner:email())

	t.query [[
		mutation {
			createTeam(
				input: {
					slug: "alpha"
					purpose: "Team that owns secrets"
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

-- Team owner creates a secret
Test.gql("Team owner creates secret with values", function(t)
	t.addHeader("x-user-email", teamOwner:email())

	t.query [[
		mutation {
			createSecret(input: {
				name: "alpha-secret"
				environment: "dev"
				team: "alpha"
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
					name = "alpha-secret",
				},
			},
		},
	}

	-- Add some values
	t.query [[
		mutation {
			addSecretValue(input: {
				name: "alpha-secret"
				environment: "dev"
				team: "alpha"
				value: {
					name: "api-key"
					value: "super-secret-value-123"
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
					name = "alpha-secret",
					keys = { "api-key" },
				},
			},
		},
	}
end)

-- ============================================================
-- Cross-team READ tests (should be allowed for metadata/keys)
-- ============================================================

Test.gql("Other team CAN see secret names via team.secrets", function(t)
	t.addHeader("x-user-email", otherUser:email())

	t.query [[
		query {
			team(slug: "alpha") {
				secrets {
					nodes {
						name
					}
				}
			}
		}
	]]

	-- Should return secrets list (may be empty if auth blocks, or have secrets if allowed)
	-- Current behavior: returns nil/empty due to CanReadSecrets check
	-- Expected new behavior: returns secret names
	t.check {
		data = {
			team = {
				secrets = {
					nodes = {
						{
							name = "alpha-secret",
						},
					},
				},
			},
		},
	}
end)

Test.gql("Other team CAN see secret keys", function(t)
	t.addHeader("x-user-email", otherUser:email())

	t.query [[
		query {
			team(slug: "alpha") {
				environment(name: "dev") {
					secret(name: "alpha-secret") {
						name
						keys
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
						name = "alpha-secret",
						keys = { "api-key" },
					},
				},
			},
		},
	}
end)

-- ============================================================
-- Cross-team VALUE read tests (should be BLOCKED)
-- ============================================================

Test.gql("Other team CANNOT see secret values via viewSecretValues", function(t)
	t.addHeader("x-user-email", otherUser:email())

	t.query [[
		mutation {
			viewSecretValues(input: {
				name: "alpha-secret"
				environment: "dev"
				team: "alpha"
				reason: "Trying to read secret values from another team"
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

-- ============================================================
-- Cross-team MUTATION tests (should all be BLOCKED)
-- ============================================================

Test.gql("Other team CANNOT create secret in alpha", function(t)
	t.addHeader("x-user-email", otherUser:email())

	t.query [[
		mutation {
			createSecret(input: {
				name: "malicious-secret"
				environment: "dev"
				team: "alpha"
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
				message = Contains("You are authenticated"),
				path = { "createSecret" },
			},
		},
		data = Null,
	}
end)

Test.gql("Other team CANNOT add value to alpha secret", function(t)
	t.addHeader("x-user-email", otherUser:email())

	t.query [[
		mutation {
			addSecretValue(input: {
				name: "alpha-secret"
				environment: "dev"
				team: "alpha"
				value: {
					name: "malicious-key"
					value: "malicious-value"
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
				message = Contains("You are authenticated"),
				path = { "addSecretValue" },
			},
		},
		data = Null,
	}
end)

Test.gql("Other team CANNOT update value in alpha secret", function(t)
	t.addHeader("x-user-email", otherUser:email())

	t.query [[
		mutation {
			updateSecretValue(input: {
				name: "alpha-secret"
				environment: "dev"
				team: "alpha"
				value: {
					name: "api-key"
					value: "hacked-value"
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
				message = Contains("You are authenticated"),
				path = { "updateSecretValue" },
			},
		},
		data = Null,
	}
end)

Test.gql("Other team CANNOT remove value from alpha secret", function(t)
	t.addHeader("x-user-email", otherUser:email())

	t.query [[
		mutation {
			removeSecretValue(input: {
				secretName: "alpha-secret"
				environment: "dev"
				team: "alpha"
				valueName: "api-key"
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
				message = Contains("You are authenticated"),
				path = { "removeSecretValue" },
			},
		},
		data = Null,
	}
end)

Test.gql("Other team CANNOT delete alpha secret", function(t)
	t.addHeader("x-user-email", otherUser:email())

	t.query [[
		mutation {
			deleteSecret(input: {
				name: "alpha-secret"
				environment: "dev"
				team: "alpha"
			}) {
				secretDeleted
			}
		}
	]]

	t.check {
		errors = {
			{
				message = Contains("You are authenticated"),
				path = { "deleteSecret" },
			},
		},
		data = Null,
	}
end)

-- ============================================================
-- Verify team owner still has full access
-- ============================================================

Test.gql("Team owner CAN see values via viewSecretValues", function(t)
	t.addHeader("x-user-email", teamOwner:email())

	t.query [[
		mutation {
			viewSecretValues(input: {
				name: "alpha-secret"
				environment: "dev"
				team: "alpha"
				reason: "Testing team owner access to secret values"
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
						name = "api-key",
						value = "super-secret-value-123",
					},
				},
			},
		},
	}
end)

Test.gql("Team owner CAN delete secret (cleanup)", function(t)
	t.addHeader("x-user-email", teamOwner:email())

	t.query [[
		mutation {
			deleteSecret(input: {
				name: "alpha-secret"
				environment: "dev"
				team: "alpha"
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
