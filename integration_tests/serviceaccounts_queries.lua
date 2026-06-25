local admin = User.new("admin", "admin@example.com", "admin-ext-id")
admin:admin(true)

local user = User.new("user", "user@example.com", "user-ext-id")

local team = Team.new("query-team", "Team for SA query tests", "#sa-queries")
team:addOwner(user)

-- Create a dummy service account that we will delete later to get a valid but non-existent ID
Test.gql("Create dummy service account to delete", function(t)
	t.addHeader("x-user-email", admin:email())

	t.query [[
		mutation {
			createServiceAccount(
				input: {
					name: "dummy-sa"
					description: "will be deleted"
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
					id = Save("deletedSaID"),
				},
			},
		},
	}
end)

Test.gql("Delete dummy service account", function(t)
	t.addHeader("x-user-email", admin:email())

	t.query(string.format([[
		mutation {
			deleteServiceAccount(input: { serviceAccountID: "%s" }) {
				serviceAccountDeleted
			}
		}
	]], State.deletedSaID))

	t.check {
		data = {
			deleteServiceAccount = {
				serviceAccountDeleted = true,
			},
		},
	}
end)

-- Create a global service account
Test.gql("Create global service account", function(t)
	t.addHeader("x-user-email", admin:email())

	t.query [[
		mutation {
			createServiceAccount(
				input: {
					name: "global-sa"
					description: "A global service account"
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
					id = Save("globalSaID"),
				},
			},
		},
	}
end)

-- Assign a role to the global SA for later verification
Test.gql("Assign role to global service account", function(t)
	t.addHeader("x-user-email", admin:email())

	t.query(string.format([[
		mutation {
			assignRoleToServiceAccount(
				input: {
					serviceAccountID: "%s"
					roleName: "Team creator"
				}
			) {
				serviceAccount {
					id
				}
			}
		}
	]], State.globalSaID))

	t.check {
		data = {
			assignRoleToServiceAccount = {
				serviceAccount = {
					id = State.globalSaID,
				},
			},
		},
	}
end)

-- Create a team service account
Test.gql("Create team service account", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		mutation {
			createServiceAccount(
				input: {
					name: "team-sa"
					description: "A team service account"
					teamSlug: "query-team"
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
					id = Save("teamSaID"),
				},
			},
		},
	}
end)

-- Assign a role to the team SA for later verification
Test.gql("Assign role to team service account", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		mutation {
			assignRoleToServiceAccount(
				input: {
					serviceAccountID: "%s"
					roleName: "Deploy key viewer"
				}
			) {
				serviceAccount {
					id
				}
			}
		}
	]], State.teamSaID))

	t.check {
		data = {
			assignRoleToServiceAccount = {
				serviceAccount = {
					id = State.teamSaID,
				},
			},
		},
	}
end)

-- Query single global service account by ID with full field coverage
Test.gql("Query global service account by ID with all fields", function(t)
	t.addHeader("x-user-email", admin:email())

	t.query(string.format([[
		query {
			serviceAccount(id: "%s") {
				id
				name
				description
				createdAt
				updatedAt
				lastUsedAt
				team {
					slug
				}
				roles {
					nodes {
						name
					}
				}
			}
		}
	]], State.globalSaID))

	t.check {
		data = {
			serviceAccount = {
				id = State.globalSaID,
				name = "global-sa",
				description = "A global service account",
				createdAt = NotNull(),
				updatedAt = NotNull(),
				lastUsedAt = Null,
				team = Null,
				roles = {
					nodes = {
						{
							name = "Team creator",
						},
					},
				},
			},
		},
	}
end)

-- Query team service account by ID with full field coverage
Test.gql("Query team service account by ID with all fields", function(t)
	t.addHeader("x-user-email", user:email())

	t.query(string.format([[
		query {
			serviceAccount(id: "%s") {
				id
				name
				description
				createdAt
				updatedAt
				lastUsedAt
				team {
					slug
					purpose
					slackChannel
				}
				roles {
					nodes {
						name
					}
				}
			}
		}
	]], State.teamSaID))

	t.check {
		data = {
			serviceAccount = {
				id = State.teamSaID,
				name = "team-sa",
				description = "A team service account",
				createdAt = NotNull(),
				updatedAt = NotNull(),
				lastUsedAt = Null,
				team = {
					slug = "query-team",
					purpose = "Team for SA query tests",
					slackChannel = "#sa-queries",
				},
				roles = {
					nodes = {
						{
							name = "Deploy key viewer",
						},
					},
				},
			},
		},
	}
end)

-- Query non-existent service account by ID (using a previously deleted SA's ID)
Test.gql("Query non-existent service account by ID", function(t)
	t.addHeader("x-user-email", admin:email())

	t.query(string.format([[
		query {
			serviceAccount(id: "%s") {
				id
				name
			}
		}
	]], State.deletedSaID))

	t.check {
		data = Null,
		errors = {
			{
				locations = NotNull(),
				message = "The specified service account was not found.",
				path = {
					"serviceAccount",
				},
			},
		},
	}
end)

-- List global service accounts
Test.gql("List global service accounts", function(t)
	t.addHeader("x-user-email", admin:email())

	t.query [[
		query {
			serviceAccounts(first: 50) {
				nodes {
					id
					name
					description
					team {
						slug
					}
				}
				pageInfo {
					totalCount
					hasNextPage
					hasPreviousPage
				}
			}
		}
	]]

	t.check {
		data = {
			serviceAccounts = {
				nodes = {
					{
						id = State.globalSaID,
						name = "global-sa",
						description = "A global service account",
						team = Null,
					},
				},
				pageInfo = {
					totalCount = 1,
					hasNextPage = false,
					hasPreviousPage = false,
				},
			},
		},
	}
end)

-- List team service accounts via team query
Test.gql("List team service accounts via team query", function(t)
	t.addHeader("x-user-email", user:email())

	t.query [[
		query {
			team(slug: "query-team") {
				serviceAccounts(first: 50) {
					nodes {
						id
						name
						description
						createdAt
						updatedAt
						lastUsedAt
						roles {
							nodes {
								name
							}
						}
					}
					pageInfo {
						totalCount
						hasNextPage
						hasPreviousPage
					}
				}
			}
		}
	]]

	t.check {
		data = {
			team = {
				serviceAccounts = {
					nodes = {
						{
							id = State.teamSaID,
							name = "team-sa",
							description = "A team service account",
							createdAt = NotNull(),
							updatedAt = NotNull(),
							lastUsedAt = Null,
							roles = {
								nodes = {
									{
										name = "Deploy key viewer",
									},
								},
							},
						},
					},
					pageInfo = {
						totalCount = 1,
						hasNextPage = false,
						hasPreviousPage = false,
					},
				},
			},
		},
	}
end)

-- Error case: Create SA with non-existent team slug
Test.gql("Create service account with non-existent team slug", function(t)
	t.addHeader("x-user-email", admin:email())

	t.query [[
		mutation {
			createServiceAccount(
				input: {
					name: "bad-sa"
					description: "should fail"
					teamSlug: "non-existent-team"
				}
			) {
				serviceAccount {
					id
				}
			}
		}
	]]

	t.check {
		data = Null,
		errors = {
			{
				locations = NotNull(),
				message = Contains("The specified team was not found."),
				path = {
					"createServiceAccount",
				},
			},
		},
	}
end)

-- Error case: Update non-existent service account (using deleted SA's ID)
Test.gql("Update non-existent service account", function(t)
	t.addHeader("x-user-email", admin:email())

	t.query(string.format([[
		mutation {
			updateServiceAccount(
				input: {
					serviceAccountID: "%s"
					description: "should fail"
				}
			) {
				serviceAccount {
					id
				}
			}
		}
	]], State.deletedSaID))

	t.check {
		data = Null,
		errors = {
			{
				locations = NotNull(),
				message = "The specified service account was not found.",
				path = {
					"updateServiceAccount",
				},
			},
		},
	}
end)

-- Error case: Delete non-existent service account (using deleted SA's ID)
Test.gql("Delete non-existent service account", function(t)
	t.addHeader("x-user-email", admin:email())

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
	]], State.deletedSaID))

	t.check {
		data = Null,
		errors = {
			{
				locations = NotNull(),
				message = "The specified service account was not found.",
				path = {
					"deleteServiceAccount",
				},
			},
		},
	}
end)

-- Error case: Assign role to non-existent service account
Test.gql("Assign role to non-existent service account", function(t)
	t.addHeader("x-user-email", admin:email())

	t.query(string.format([[
		mutation {
			assignRoleToServiceAccount(
				input: {
					serviceAccountID: "%s"
					roleName: "Team creator"
				}
			) {
				serviceAccount {
					id
				}
			}
		}
	]], State.deletedSaID))

	t.check {
		data = Null,
		errors = {
			{
				locations = NotNull(),
				message = "The specified service account was not found.",
				path = {
					"assignRoleToServiceAccount",
				},
			},
		},
	}
end)

-- Error case: Revoke role from non-existent service account
Test.gql("Revoke role from non-existent service account", function(t)
	t.addHeader("x-user-email", admin:email())

	t.query(string.format([[
		mutation {
			revokeRoleFromServiceAccount(
				input: {
					serviceAccountID: "%s"
					roleName: "Team creator"
				}
			) {
				serviceAccount {
					id
				}
			}
		}
	]], State.deletedSaID))

	t.check {
		data = Null,
		errors = {
			{
				locations = NotNull(),
				message = "The specified service account was not found.",
				path = {
					"revokeRoleFromServiceAccount",
				},
			},
		},
	}
end)
