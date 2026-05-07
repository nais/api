local admin = User.new("admin-user", "admin@example.com", "admin-ext-id")
admin:admin(true)

local unprivilegedUser = User.new("unprivileged", "unprivileged@example.com", "unpriv-ext-id")

-- Create a service account to work with
Test.gql("Create service account for token tests", function(t)
	t.addHeader("x-user-email", admin:email())

	t.query [[
		mutation {
			createServiceAccount(
				input: {
					name: "token-test-sa"
					description: "SA for comprehensive token testing"
				}
			) {
				serviceAccount {
					id
					name
				}
			}
		}
	]]

	t.check {
		data = {
			createServiceAccount = {
				serviceAccount = {
					id = Save("saID"),
					name = "token-test-sa",
				},
			},
		},
	}
end)

-- Test: List tokens on a fresh SA (should be empty)
Test.gql("List tokens on service account with no tokens", function(t)
	t.addHeader("x-user-email", admin:email())

	t.query(string.format([[
		query {
			serviceAccount(id: "%s") {
				tokens(first: 10) {
					nodes {
						id
						name
					}
					pageInfo {
						totalCount
						hasNextPage
					}
				}
			}
		}
	]], State.saID))

	t.check {
		data = {
			serviceAccount = {
				tokens = {
					nodes = {},
					pageInfo = {
						totalCount = 0,
						hasNextPage = false,
					},
				},
			},
		},
	}
end)

-- Test: Create token without expiry date (expiresAt should be null)
Test.gql("Create token without expiry date", function(t)
	t.addHeader("x-user-email", admin:email())

	t.query(string.format([[
		mutation {
			createServiceAccountToken(
				input: {
					serviceAccountID: "%s"
					name: "no-expiry-token"
					description: "Token without expiry"
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
					createdAt
					updatedAt
					lastUsedAt
					expiresAt
				}
			}
		}
	]], State.saID))

	t.check {
		data = {
			createServiceAccountToken = {
				secret = Save("tokenSecret1"),
				serviceAccount = {
					id = State.saID,
				},
				serviceAccountToken = {
					id = Save("tokenID1"),
					name = "no-expiry-token",
					description = "Token without expiry",
					createdAt = NotNull(),
					updatedAt = NotNull(),
					lastUsedAt = Null,
					expiresAt = Null,
				},
			},
		},
	}
end)

-- Test: Verify token secret format starts with "nais_console_"
Test.gql("Create token and verify secret format", function(t)
	t.addHeader("x-user-email", admin:email())

	t.query(string.format([[
		mutation {
			createServiceAccountToken(
				input: {
					serviceAccountID: "%s"
					name: "format-check-token"
					description: "Token for secret format verification"
					expiresAt: "2030-06-15"
				}
			) {
				secret
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
				serviceAccountToken = {
					id = Save("tokenID2"),
					name = "format-check-token",
					description = "Token for secret format verification",
					expiresAt = "2030-06-15",
				},
			},
		},
	}
end)

-- Test: Create a third token on the same SA
Test.gql("Create a third token on the same SA", function(t)
	t.addHeader("x-user-email", admin:email())

	t.query(string.format([[
		mutation {
			createServiceAccountToken(
				input: {
					serviceAccountID: "%s"
					name: "third-token"
					description: "Third token for listing"
					expiresAt: "2028-12-31"
				}
			) {
				secret
				serviceAccountToken {
					id
					name
				}
			}
		}
	]], State.saID))

	t.check {
		data = {
			createServiceAccountToken = {
				secret = Contains("nais_console_"),
				serviceAccountToken = {
					id = Save("tokenID3"),
					name = "third-token",
				},
			},
		},
	}
end)

-- Test: List all tokens on the SA (should have 3)
Test.gql("List all tokens on service account with multiple tokens", function(t)
	t.addHeader("x-user-email", admin:email())

	t.query(string.format([[
		query {
			serviceAccount(id: "%s") {
				tokens(first: 10) {
					nodes {
						id
						name
						description
						createdAt
						updatedAt
						lastUsedAt
						expiresAt
					}
					pageInfo {
						totalCount
						hasNextPage
					}
				}
			}
		}
	]], State.saID))

	t.check {
		data = {
			serviceAccount = {
				tokens = {
					nodes = {
						{
							id = State.tokenID2,
							name = "format-check-token",
							description = "Token for secret format verification",
							createdAt = NotNull(),
							updatedAt = NotNull(),
							lastUsedAt = Null,
							expiresAt = "2030-06-15",
						},
						{
							id = State.tokenID1,
							name = "no-expiry-token",
							description = "Token without expiry",
							createdAt = NotNull(),
							updatedAt = NotNull(),
							lastUsedAt = Null,
							expiresAt = Null,
						},
						{
							id = State.tokenID3,
							name = "third-token",
							description = "Third token for listing",
							createdAt = NotNull(),
							updatedAt = NotNull(),
							lastUsedAt = Null,
							expiresAt = "2028-12-31",
						},
					},
					pageInfo = {
						totalCount = 3,
						hasNextPage = false,
					},
				},
			},
		},
	}
end)

-- Test: Update token with name change only
Test.gql("Update token name only", function(t)
	t.addHeader("x-user-email", admin:email())

	t.query(string.format([[
		mutation {
			updateServiceAccountToken(
				input: {
					serviceAccountTokenID: "%s"
					name: "renamed-token"
				}
			) {
				serviceAccount {
					id
				}
				serviceAccountToken {
					id
					name
					description
				}
			}
		}
	]], State.tokenID1))

	t.check {
		data = {
			updateServiceAccountToken = {
				serviceAccount = {
					id = State.saID,
				},
				serviceAccountToken = {
					id = State.tokenID1,
					name = "renamed-token",
					description = "Token without expiry",
				},
			},
		},
	}
end)

-- Test: Update token with both name and description
Test.gql("Update token name and description simultaneously", function(t)
	t.addHeader("x-user-email", admin:email())

	t.query(string.format([[
		mutation {
			updateServiceAccountToken(
				input: {
					serviceAccountTokenID: "%s"
					name: "fully-updated-token"
					description: "Updated description"
				}
			) {
				serviceAccountToken {
					id
					name
					description
				}
			}
		}
	]], State.tokenID2))

	t.check {
		data = {
			updateServiceAccountToken = {
				serviceAccountToken = {
					id = State.tokenID2,
					name = "fully-updated-token",
					description = "Updated description",
				},
			},
		},
	}
end)

-- Test: Update token without permission (unprivileged user)
Test.gql("Update token without permission", function(t)
	t.addHeader("x-user-email", unprivilegedUser:email())

	t.query(string.format([[
		mutation {
			updateServiceAccountToken(
				input: {
					serviceAccountTokenID: "%s"
					description: "hacker description"
				}
			) {
				serviceAccountToken {
					id
				}
			}
		}
	]], State.tokenID1))

	t.check {
		data = Null,
		errors = {
			{
				locations = NotNull(),
				message = Contains("service_accounts:update"),
				path = {
					"updateServiceAccountToken",
				},
			},
		},
	}
end)

-- Test: Delete token without permission (unprivileged user)
Test.gql("Delete token without permission", function(t)
	t.addHeader("x-user-email", unprivilegedUser:email())

	t.query(string.format([[
		mutation {
			deleteServiceAccountToken(input: { serviceAccountTokenID: "%s" }) {
				serviceAccountTokenDeleted
			}
		}
	]], State.tokenID3))

	t.check {
		data = Null,
		errors = {
			{
				locations = NotNull(),
				message = Contains("service_accounts:update"),
				path = {
					"deleteServiceAccountToken",
				},
			},
		},
	}
end)

-- Test: Token is usable for authentication (Bearer header)
local bearerHeader = string.format("Bearer %s", State.tokenSecret1)

Test.gql("Authenticate as service account using token", function(t)
	t.addHeader("authorization", bearerHeader)

	t.query [[
		query {
			me {
				... on ServiceAccount {
					name
				}
			}
		}
	]]

	t.check {
		data = {
			me = {
				name = "token-test-sa",
			},
		},
	}
end)

-- Test: Delete a token and verify list count decreases
Test.gql("Delete third token", function(t)
	t.addHeader("x-user-email", admin:email())

	t.query(string.format([[
		mutation {
			deleteServiceAccountToken(input: { serviceAccountTokenID: "%s" }) {
				serviceAccount {
					id
				}
				serviceAccountTokenDeleted
			}
		}
	]], State.tokenID3))

	t.check {
		data = {
			deleteServiceAccountToken = {
				serviceAccount = {
					id = State.saID,
				},
				serviceAccountTokenDeleted = true,
			},
		},
	}
end)

Test.gql("List tokens after deletion shows reduced count", function(t)
	t.addHeader("x-user-email", admin:email())

	t.query(string.format([[
		query {
			serviceAccount(id: "%s") {
				tokens(first: 10) {
					pageInfo {
						totalCount
					}
				}
			}
		}
	]], State.saID))

	t.check {
		data = {
			serviceAccount = {
				tokens = {
					pageInfo = {
						totalCount = 2,
					},
				},
			},
		},
	}
end)

-- Test: Create token for non-existent service account (using the stale saID after deletion later)
-- First, let's delete the remaining tokens and SA, then try creating a token on the deleted SA
Test.gql("Delete first token", function(t)
	t.addHeader("x-user-email", admin:email())

	t.query(string.format([[
		mutation {
			deleteServiceAccountToken(input: { serviceAccountTokenID: "%s" }) {
				serviceAccountTokenDeleted
			}
		}
	]], State.tokenID1))

	t.check {
		data = {
			deleteServiceAccountToken = {
				serviceAccountTokenDeleted = true,
			},
		},
	}
end)

Test.gql("Delete second token", function(t)
	t.addHeader("x-user-email", admin:email())

	t.query(string.format([[
		mutation {
			deleteServiceAccountToken(input: { serviceAccountTokenID: "%s" }) {
				serviceAccountTokenDeleted
			}
		}
	]], State.tokenID2))

	t.check {
		data = {
			deleteServiceAccountToken = {
				serviceAccountTokenDeleted = true,
			},
		},
	}
end)

Test.gql("Delete service account", function(t)
	t.addHeader("x-user-email", admin:email())

	t.query(string.format([[
		mutation {
			deleteServiceAccount(input: { serviceAccountID: "%s" }) {
				serviceAccountDeleted
			}
		}
	]], State.saID))

	t.check {
		data = {
			deleteServiceAccount = {
				serviceAccountDeleted = true,
			},
		},
	}
end)

-- Test: Create token for deleted service account
Test.gql("Create token for deleted service account", function(t)
	t.addHeader("x-user-email", admin:email())

	t.query(string.format([[
		mutation {
			createServiceAccountToken(
				input: {
					serviceAccountID: "%s"
					name: "stale-token"
					description: "should fail"
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
				locations = NotNull(),
				message = "The specified service account was not found.",
				path = {
					"createServiceAccountToken",
				},
			},
		},
	}
end)
