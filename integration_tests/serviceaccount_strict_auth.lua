Helper.readK8sResources("k8s_resources/secrets")

-- Create a user who will own the team and set up the secret
local owner = User.new("sa-owner", "sa-owner@example.com", "ext-sa-owner")
local team = Team.new("myteam", "Team for SA strict auth test", "#sa-test")
team:addOwner(owner)

-- Create a second team for cross-team tests
local otherTeam = Team.new("otherteam", "Other team", "#other")

-- Create a team-scoped service account with "Team member" role
local sa = ServiceAccount.new("strict-auth-sa", "myteam")
sa:assignRole("Team member")

-- Create a cross-team service account (belongs to otherteam)
local crossTeamSa = ServiceAccount.new("cross-team-sa", "otherteam")
crossTeamSa:assignRole("Team member")

-- Create a global service account (no team)
local globalSa = ServiceAccount.new("global-sa")
globalSa:assignRole("Team member")

-- Owner creates a secret with a value to test reading
Test.gql("Owner creates secret with value", function(t)
	t.addHeader("x-user-email", owner:email())

	t.query [[
		mutation {
			createSecret(input: {
				name: "sa-test-secret"
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
					name = "sa-test-secret",
				},
			},
		},
	}

	t.query [[
		mutation {
			addSecretValue(input: {
				name: "sa-test-secret"
				environment: "dev"
				team: "myteam"
				value: {
					name: "MY_KEY"
					value: "my-secret-value"
				}
			}) {
				secret {
					name
				}
			}
		}
	]]

	t.check {
		data = {
			addSecretValue = {
				secret = {
					name = "sa-test-secret",
				},
			},
		},
	}
end)

-- Service account should be able to read secret values.
-- This uses requireStrictTeamAuthorization (CanReadSecretValues),
-- which was broken before the ServiceAccountHasTeamMembership fix.
Test.gql("Service account CAN read secret values (strict auth)", function(t)
	t.addHeader("Authorization", "Bearer " .. sa:token())

	t.query [[
		mutation {
			viewSecretValues(input: {
				name: "sa-test-secret"
				environment: "dev"
				team: "myteam"
				reason: "Service account reading secret values for automation"
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
						name = "MY_KEY",
						value = "my-secret-value",
					},
				},
			},
		},
	}
end)

-- A service account scoped to a DIFFERENT team should NOT be able
-- to read secrets, even with "Team member" role.
-- This verifies that the strict check enforces team_slug matching.
Test.gql("Cross-team SA CANNOT read secret values (strict auth)", function(t)
	t.addHeader("Authorization", "Bearer " .. crossTeamSa:token())

	t.query [[
		mutation {
			viewSecretValues(input: {
				name: "sa-test-secret"
				environment: "dev"
				team: "myteam"
				reason: "Cross-team SA trying to read secrets from another team"
			}) {
				values {
					name
					value
				}
			}
		}
	]]

	t.check {
		data = Null,
		errors = {
			{
				locations = NotNull(),
				message = Contains("You are authenticated"),
				path = { "viewSecretValues" },
			},
		},
	}
end)

-- A global service account (no team) should also NOT pass strict checks.
Test.gql("Global SA CANNOT read secret values (strict auth rejects global)", function(t)
	t.addHeader("Authorization", "Bearer " .. globalSa:token())

	t.query [[
		mutation {
			viewSecretValues(input: {
				name: "sa-test-secret"
				environment: "dev"
				team: "myteam"
				reason: "Global SA trying to read secrets via strict auth check"
			}) {
				values {
					name
					value
				}
			}
		}
	]]

	t.check {
		data = Null,
		errors = {
			{
				locations = NotNull(),
				message = Contains("You are authenticated"),
				path = { "viewSecretValues" },
			},
		},
	}
end)
