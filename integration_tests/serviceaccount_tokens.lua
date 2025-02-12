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
					id: "%s"
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
					note: "some note"
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
					note: "some note"
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
					note: "some note"
				}
			) {
				secret
				serviceAccount {
					id
				}
				serviceAccountToken {
					id
					note
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
					note = "some note",
				},
			},
		},
	}
end)
