local user = User.new("username", "user@example.com", "e")
user:admin(true)

local userWithoutPermission = User.new("username-1", "user1@example.com", "e2")


Test.gql("Create dummy service account", function(t)
	t.addHeader("x-user-email", user:email())

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
	t.addHeader("x-user-email", user:email())

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
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		mutation {
			createServiceAccountToken(
				input: {
					serviceAccountID: "%s"
					name: "my-token"
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
	t.addHeader("x-user-email", user:email())

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
	t.addHeader("x-user-email", userWithoutPermission:email())

	t.query(string.format([[
		mutation {
			createServiceAccountToken(
				input: {
					serviceAccountID: "%s"
					name: "my-token"
					description: "some description"
				}
			) {
				serviceAccountToken {
					id
				}
			}
		}
	]], State.saID))

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
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		mutation {
			createServiceAccountToken(
				input: {
					serviceAccountID: "%s"
					name: "my-token"
					description: "some description"
					expiresAt: "2029-01-01"
				}
			) {
				secret
				serviceAccount {
					id
				}
				serviceAccountToken {
					id
					name
					description
					expiresAt
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
					name = "my-token",
					description = "some description",
					expiresAt = "2029-01-01",
				},
			},
		},
	}
end)

Test.gql("Update service account token", function(t)
	t.addHeader("x-user-email", user:email())

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
		data = {
			updateServiceAccountToken = {
				serviceAccountToken = {
					id = State.tokenID,
					description = "some other description",
					expiresAt = "2029-01-01",
				},
			},
		},
	}
end)

Test.gql("Delete service account token", function(t)
	t.addHeader("x-user-email", user:email())

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
	t.addHeader("x-user-email", user:email())

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
	t.addHeader("x-user-email", user:email())

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
