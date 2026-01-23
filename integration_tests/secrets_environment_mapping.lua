-- Test secrets with environment mapping enabled
-- This tests the scenario used by NAV.no where cluster names (dev) are mapped to user-facing names (dev-gcp)
--
-- Environment mapping is configured in secrets_environment_mapping.yaml:
--   dev -> dev-gcp
--   staging -> staging-gcp
--
-- This test verifies that all secret operations work correctly when users send the mapped
-- environment name (dev-gcp) instead of the cluster name (dev).
--
-- Before the fix, these operations would fail with "no watcher for cluster dev" because:
-- 1. GraphQL resolvers would map dev-gcp -> dev
-- 2. Queries would try to find watcher for "dev"
-- 3. But watchers store cluster names as "dev-gcp" (mapped names)
--
-- After the fix, environment names are passed through unchanged, matching watcher setup.

Helper.readK8sResources("k8s_resources/secrets")

local user = User.new("username-mapping", "user-mapping@example.com", "em")
local team = Team.new("mappingteam", "team for testing environment mapping", "#channel")
team:addOwner(user)

Test.gql("Create secret with environment mapping", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			createSecret(input: {
				name: "mapped-secret"
				environment: "dev-gcp"
				team: "mappingteam"
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
					name = "mapped-secret",
					keys = {},
				},
			},
		},
	}
end)

Test.gql("Add secret value with environment mapping", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			addSecretValue(input: {
				name: "mapped-secret"
				environment: "dev-gcp"
				team: "mappingteam"
				value: {
					name: "API_KEY",
					value: "super-secret-value"
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
					name = "mapped-secret",
					keys = { "API_KEY" },
				},
			},
		},
	}
end)

Test.gql("Add another secret value with environment mapping", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			addSecretValue(input: {
				name: "mapped-secret"
				environment: "dev-gcp"
				team: "mappingteam"
				value: {
					name: "DATABASE_URL",
					value: "postgres://localhost/db"
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
					name = "mapped-secret",
					keys = { "API_KEY", "DATABASE_URL" },
				},
			},
		},
	}
end)

Test.gql("Update secret value with environment mapping", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			updateSecretValue(input: {
				name: "mapped-secret"
				environment: "dev-gcp"
				team: "mappingteam"
				value: {
					name: "API_KEY",
					value: "new-super-secret-value"
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
					name = "mapped-secret",
					keys = { "API_KEY", "DATABASE_URL" },
				},
			},
		},
	}
end)

Test.gql("List secrets with environment mapping", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "mappingteam") {
				secrets {
					nodes {
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
				secrets = {
					nodes = {
						{
							name = "mapped-secret",
							keys = { "API_KEY", "DATABASE_URL" },
						},
					},
				},
			},
		},
	}
end)

Test.gql("Get secret by name with environment mapping", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "mappingteam") {
				environment(name: "dev-gcp") {
					secret(name: "mapped-secret") {
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
						name = "mapped-secret",
						keys = { "API_KEY", "DATABASE_URL" },
					},
				},
			},
		},
	}
end)

Test.gql("Create elevation for reading secret values with environment mapping", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			createElevation(input: {
				type: SECRET
				team: "mappingteam"
				environmentName: "dev-gcp"
				resourceName: "mapped-secret"
				reason: "Testing secret values access with environment mapping"
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
					id = Save("mappingElevationID"),
				},
			},
		},
	}
end)

Test.gql("Read secret values with elevation and environment mapping", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "mappingteam") {
				environment(name: "dev-gcp") {
					secret(name: "mapped-secret") {
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
						name = "mapped-secret",
						values = {
							{
								name = "API_KEY",
								value = "new-super-secret-value",
							},
							{
								name = "DATABASE_URL",
								value = "postgres://localhost/db",
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("Remove secret value with environment mapping", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			removeSecretValue(input: {
				secretName: "mapped-secret"
				environment: "dev-gcp"
				team: "mappingteam"
				valueName: "DATABASE_URL"
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
					name = "mapped-secret",
					keys = { "API_KEY" },
				},
			},
		},
	}
end)

Test.gql("Delete secret with environment mapping", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			deleteSecret(input: {
				name: "mapped-secret"
				environment: "dev-gcp"
				team: "mappingteam"
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

Test.gql("Verify secret is deleted with environment mapping", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "mappingteam") {
				environment(name: "dev-gcp") {
					secret(name: "mapped-secret") {
						name
					}
				}
			}
		}
	]]

	t.check {
		errors = {
			{
				message = Contains("Resource not found"),
				path = { "team", "environment", "secret" },
			},
		},
		data = Null,
	}
end)
