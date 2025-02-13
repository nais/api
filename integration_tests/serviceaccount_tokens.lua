Helper.SQLExec [[
	UPDATE users SET admin = true WHERE email = 'authenticated@example.com'
]]

Test.gql("Create dummy service account", function(t)
	t.query [[
		mutation {
			createServiceAccount(
				input: {
					name: "dummy"
					description: "some description"
				}
			) {
				serviceAccount {
					id
				}
			}
		}
	]]

	t.check {
		data = {
			createServiceAccount = {
				serviceAccount = {
					id = Save("dummyID"),
				},
			},
		},
	}
end)

Test.gql("Delete dummy service account", function(t)
	t.query(string.format([[
		mutation {
			deleteServiceAccount(
				input: {
					serviceAccountID: "%s"
				}
			) {
				serviceAccountDeleted
			}
		}
	]], State.dummyID))

	t.check {
		data = {
			deleteServiceAccount = {
				serviceAccountDeleted = true,
			},
		},
	}
end)

Test.gql("Create token for service account that does not exist", function(t)
	t.query(string.format([[
		mutation {
			createServiceAccountToken(
				input: {
					serviceAccountID: "%s"
					description: "some description"
				}
			) {
				serviceAccountToken {
					id
				}
			}
		}
	]], State.dummyID))

	t.check {
		data = Null,
		errors = {
			{
				message = "The specified service account was not found.",
				path = {
					"createServiceAccountToken",
				},
			},
		},
	}
end)

Test.gql("Create service account", function(t)
	t.query [[
		mutation {
			createServiceAccount(
				input: {
					name: "sa"
					description: "some description"
				}
			) {
				serviceAccount {
					id
				}
			}
		}
	]]

	t.check {
		data = {
			createServiceAccount = {
				serviceAccount = {
					id = Save("saID"),
				},
			},
		},
	}
end)

Test.gql("Create service account token without permission", function(t)
	t.query(string.format([[
		mutation {
			createServiceAccountToken(
				input: {
					serviceAccountID: "%s"
					description: "some description"
				}
			) {
				serviceAccountToken {
					id
				}
			}
		}
	]], State.saID), { ["x-user-email"] = "email-1@example.com" })

	t.check {
		data = Null,
		errors = {
			{
				message = Contains("Specifically, you need the \"service_accounts:update\" authorization."),
				path = {
					"createServiceAccountToken",
				},
			},
		},
	}
end)

Test.gql("Create service account token", function(t)
	t.query(string.format([[
		mutation {
			createServiceAccountToken(
				input: {
					serviceAccountID: "%s"
					description: "some description"
				}
			) {
				secret
				serviceAccount {
					id
				}
				serviceAccountToken {
					id
					description
				}
			}
		}
	]], State.saID))

	t.check {
		data = {
			createServiceAccountToken = {
				secret = Contains("nais_console_"),
				serviceAccount = {
					id = State.saID,
				},
				serviceAccountToken = {
					id = Save("tokenID"),
					description = "some description",
				},
			},
		},
	}
end)

Test.gql("Update service account token with empty expiresAt", function(t)
	t.query(string.format([[
		mutation {
			updateServiceAccountToken(
				input: {
					serviceAccountTokenID: "%s"
					expiresAt: {  }
				}
			) {
				serviceAccountToken {
					expiresAt
				}
			}
		}
	]], State.tokenID))

	t.check {
		data = Null,
		errors = {
			{
				extensions = {
					code = "GRAPHQL_VALIDATION_FAILED",
				},
				locations = NotNull(),
				message = "OneOf Input Object \"UpdateServiceAccountTokenExpiresAtInput\" must specify exactly one key.",
			},
		},
	}
end)

Test.gql("Update service account token", function(t)
	t.query(string.format([[
		mutation {
			updateServiceAccountToken(
				input: {
					serviceAccountTokenID: "%s"
					description: "some other description"
					expiresAt: { removeExpiry: true }
				}
			) {
				serviceAccountToken {
					id
					description
					expiresAt
				}
			}
		}
	]], State.tokenID))

	t.check {
		data = {
			updateServiceAccountToken = {
				serviceAccountToken = {
					id = State.tokenID,
					description = "some other description",
					expiresAt = Null,
				},
			},
		},
	}
end)

Test.gql("Update service account token set expiresAt", function(t)
	t.query(string.format([[
		mutation {
			updateServiceAccountToken(
				input: {
					serviceAccountTokenID: "%s"
					expiresAt: {
						expiresAt: "2029-04-04"
					}
				}
			) {
				serviceAccount {
					tokens {
						nodes {
							description
							expiresAt
						}
					}
				}
				serviceAccountToken {
					id
					description
					expiresAt
				}
			}
		}
	]], State.tokenID))

	t.check {
		data = {
			updateServiceAccountToken = {
				serviceAccountToken = {
					id = State.tokenID,
					description = "some other description",
					expiresAt = "2029-04-04",
				},
				serviceAccount = {
					tokens = {
						nodes = {
							{
								description = "some other description",
								expiresAt = "2029-04-04",
							},
						},
					},
				},
			},
		},
	}
end)

Test.gql("Delete service account token", function(t)
	t.query(string.format([[
		mutation {
			deleteServiceAccountToken(input: { serviceAccountTokenID: "%s" }) {
				serviceAccount {
					id
				}
			}
		}
	]], State.tokenID))

	t.check {
		data = {
			deleteServiceAccountToken = {
				serviceAccount = {
					id = State.saID,
				},
			},
		},
	}
end)

Test.gql("Update service account token that does not exist", function(t)
	t.query(string.format([[
		mutation {
			updateServiceAccountToken(
				input: {
					serviceAccountTokenID: "%s"
					description: "some other description"
				}
			) {
				serviceAccountToken {
					id
					description
					expiresAt
				}
			}
		}
	]], State.tokenID))

	t.check {
		data = Null,
		errors = {
			{
				message = Contains("Object was not found in the database."),
				path = {
					"updateServiceAccountToken",
				},
			},
		},
	}
end)

Test.gql("Delete service account token that does not exist", function(t)
	t.query(string.format([[
		mutation {
			deleteServiceAccountToken(input: { serviceAccountTokenID: "%s" }) {
				serviceAccount {
					id
				}
			}
		}
	]], State.tokenID))

	t.check {
		data = Null,
		errors = {
			{
				message = Contains("Object was not found in the database."),
				path = {
					"deleteServiceAccountToken",
				},
			},
		},
	}
end)
